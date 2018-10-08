package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/camd67/moebot/moebot_bot/util/db"
)

type ScheduleCommand struct {
	schedulers map[db.SchedulerType]Scheduler
}

func NewScheduleCommand(factory *SchedulerFactory) *ScheduleCommand {
	return &ScheduleCommand{
		schedulers: map[db.SchedulerType]Scheduler{
			db.SchedulerChannelRotation: factory.CreateScheduler(db.SchedulerChannelRotation),
		},
	}
}

func (c *ScheduleCommand) Execute(pack *CommPackage) {
	if len(pack.params) == 0 {
		c.listSchedulers(pack)
		return
	}
	if strings.ToUpper(pack.params[0]) == "LIST" {
		c.listOperations(pack)
		return
	}

	if strings.ToUpper(pack.params[0]) == "REMOVE" {
		c.removeOperation(pack)
		return
	}
	if !c.addOperation(pack) {
		pack.session.ChannelMessageSend(pack.channel.ID, "Cannot find any scheduler with the command `"+pack.params[1]+"`, please check the commands list.")
	}
}
func (c *ScheduleCommand) GetPermLevel() db.Permission {
	return db.PermMod
}
func (c *ScheduleCommand) GetCommandKeys() []string {
	return []string{"SCHEDULE"}
}
func (c *ScheduleCommand) GetCommandHelp(commPrefix string) string {
	return fmt.Sprintf("`%[1]s schedule <schedule type> <options>` - Master/Mod Creates a new scheduler with the given type and options. `%[1]s schedule` to list available schedulers.", commPrefix)
}

func (c *ScheduleCommand) listSchedulers(pack *CommPackage) {
	message := "List of available schedulers:"
	for _, sch := range c.schedulers {
		message += "\r\n"
		message += sch.Help()
	}
	message += "\r\nList - lists all active operations on the server"
	message += "\r\nRemove <operation number> - removes the operation"
	pack.session.ChannelMessageSend(pack.channel.ID, message)
}

func (c *ScheduleCommand) listOperations(pack *CommPackage) {
	server, err := db.ServerQueryOrInsert(pack.guild.ID)
	if err != nil {
		pack.session.ChannelMessageSend(pack.channel.ID, "There was a problem retrieving the current server. Please try again.")
		return
	}
	operations, err := db.ScheduledOperationQueryServer(server.Id)
	if err != nil {
		pack.session.ChannelMessageSend(pack.channel.ID, "There was a problem retrieving the current operations list. Please try again.")
		return
	}
	if len(operations) == 0 {
		pack.session.ChannelMessageSend(pack.channel.ID, "There are no operations scheduled for this server.")
		return
	}
	message := "List of active operations for the server:"
	for _, o := range operations {
		message += "\r\n"
		message += fmt.Sprintf("`%d` %s - Planned Execution: %s", o.ID, c.schedulers[o.Type].OperationDescription(o.ID), o.PlannedExecutionTime.Format(time.Stamp))
	}
	pack.session.ChannelMessageSend(pack.channel.ID, message)
}

func (c *ScheduleCommand) removeOperation(pack *CommPackage) {
	server, err := db.ServerQueryOrInsert(pack.guild.ID)
	if err != nil {
		pack.session.ChannelMessageSend(pack.channel.ID, "There was a problem retrieving the current server. Please try again.")
		return
	}
	operationID, err := strconv.ParseInt(pack.params[1], 10, 64)
	if err != nil {
		pack.session.ChannelMessageSend(pack.channel.ID, pack.params[1]+" is not a valid operation ID. Please try again.")
		return
	}
	if ok, err := db.ScheduledOperationDelete(operationID, server.Id); err != nil || !ok {
		pack.session.ChannelMessageSend(pack.channel.ID, pack.params[1]+" is not a valid operation ID. Please try again.")
		return
	}
	pack.session.ChannelMessageSend(pack.channel.ID, "Operation successfully removed.")
}

func (c *ScheduleCommand) addOperation(pack *CommPackage) bool {
	for _, s := range c.schedulers {
		if s.Keyword() == pack.params[0] {
			s.AddScheduledOperation(pack)
			return true
		}
	}
	return false
}