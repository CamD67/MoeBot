package commands

import (
	"database/sql"
	"log"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/camd67/moebot/moebot_bot/util"
	"github.com/camd67/moebot/moebot_bot/util/db"
)

type PollsHandler struct {
	pollsList []*db.Poll
}

func NewPollsHandler() *PollsHandler {
	h := new(PollsHandler)
	h.loadFromDb()
	return h
}

func (handler *PollsHandler) loadFromDb() {
	polls, _ := db.PollsOpenQuery()
	handler.pollsList = polls
}

func (handler *PollsHandler) openPoll(pack *CommPackage) {
	var options []string
	var title string
	for i := 0; i < len(pack.Params); i++ {
		if pack.Params[i] == "-options" {
			options = parseOptions(pack.Params[i+1:])
		}
		if pack.Params[i] == "-title" {
			title = parseTitle(pack.Params[i+1:])
		}
	}
	if len(options) <= 1 {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, you must specify at least two options to create a poll.")
		return
	}
	if len(options) > 25 {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there can only be a maximum of 25 options per poll.")
		return
	}
	server, err := db.ServerQueryOrInsert(pack.Guild.ID)
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem creating the poll. Please try again.")
		return
	}
	channel, err := db.ChannelQueryOrInsert(pack.Channel.ID, &server)
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem creating the poll. Please try again.")
		return
	}
	poll := &db.Poll{
		Title:     title,
		UserUid:   pack.Message.Author.ID,
		ChannelId: channel.Id,
		Open:      true,
		Options:   util.CreatePollOptions(options),
	}
	err = db.PollAdd(poll)
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem creating the poll. Please try again.")
		return
	}
	db.PollOptionAdd(poll)
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem creating the poll. Please try again.")
		return
	}
	message, _ := pack.Session.ChannelMessageSend(pack.Channel.ID, util.OpenPollMessage(poll, pack.Message.Author))
	for _, o := range poll.Options {
		err = pack.Session.MessageReactionAdd(pack.Channel.ID, message.ID, o.ReactionId)
		if err != nil {
			log.Println("Cannot add reaction to poll message", err)
		}
	}
	poll.MessageUid = message.ID
	err = db.PollSetMessageId(poll)
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem updating the poll. Please delete and create it again.")
	}
	handler.pollsList = append(handler.pollsList, poll)
}

func parseOptions(params []string) []string {
	for i := 0; i < len(params); i++ {
		if params[i][0] == '-' {
			return strings.Split(strings.Join(params[:i], " "), ",")
		}
	}
	return strings.Split(strings.Join(params, " "), ",")
}

func parseTitle(params []string) string {
	for i := 0; i < len(params); i++ {
		if params[i][0] == '-' {
			return strings.Join(params[:i], " ")
		}
	}
	return strings.Join(params, " ")
}

func (handler *PollsHandler) closePoll(pack *CommPackage) {
	if len(pack.Params) < 2 {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, you have to specify a valid ID for the poll")
		return
	}
	id, err := strconv.Atoi(pack.Params[1])
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem parsing the poll ID, please check if it's a valid ID")
		return
	}
	poll := handler.pollFromId(id)
	if poll == nil {
		poll, err = db.PollQuery(id)
		if err == sql.ErrNoRows {
			pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there is no valid poll with the given ID")
			return
		} else if err != nil {
			pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem retreiving the poll with the given ID")
			return
		}
		handler.pollsList = append(handler.pollsList, poll)
	}
	channel, err := db.ChannelQueryById(poll.ChannelId)
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem retrieving poll data")
		return
	}
	if channel.ChannelUid != pack.Channel.ID {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, you can't close a poll opened in another channel")
		return
	}
	if !poll.Open {
		pack.Session.ChannelMessageSend(pack.Channel.ID, util.ClosePollMessage(poll, pack.Message.Author))
		return
	}
	err = util.UpdatePollVotes(poll, pack.Session)
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem retrieving the votes count for the given Poll")
		return
	}
	db.PollOptionUpdateVotes(poll)
	err = db.PollClose(id)
	if err != nil {
		pack.Session.ChannelMessageSend(pack.Channel.ID, "Sorry, there was a problem closing the poll.")
		return
	}
	pack.Session.ChannelMessageSend(pack.Channel.ID, util.ClosePollMessage(poll, pack.Message.Author))
	poll.Open = false
}

func (handler *PollsHandler) pollFromId(id int) *db.Poll {
	for _, p := range handler.pollsList {
		if p.Id == id {
			return p
		}
	}
	return nil
}

func (handler *PollsHandler) checkSingleVote(session *discordgo.Session, reactionAdd *discordgo.MessageReactionAdd) {
	var err error
	for _, p := range handler.pollsList {
		if p.MessageUid == reactionAdd.MessageID {
			p.Options, err = db.PollOptionQuery(p.Id)
			if err != nil {
				log.Println("Cannot retrieve poll options informations", err)
				return
			}
			//If the user is reacting to a poll, we check if he has already cast a vote and remove it
			handler.handleSingleVote(session, p, reactionAdd)
			return
		}
	}
}

func (handler *PollsHandler) handleSingleVote(session *discordgo.Session, poll *db.Poll, reactionAdd *discordgo.MessageReactionAdd) {
	channel, err := db.ChannelQueryById(poll.ChannelId)
	if err != nil {
		log.Println("Cannot retrieve poll channel informations", err)
		return
	}
	message, err := session.ChannelMessage(channel.ChannelUid, poll.MessageUid)
	if err != nil {
		log.Println("Cannot retrieve poll message informations", err)
		return
	}
	if message.Author.ID == reactionAdd.UserID {
		return //The bot is modifying its own reactions
	}
	for _, r := range message.Reactions {
		if !reactionIsOption(poll.Options, r.Emoji.Name) {
			continue
		}
		//Getting a list of users for every reaction
		users, err := session.MessageReactions(channel.ChannelUid, poll.MessageUid, r.Emoji.Name, 100)
		if err != nil {
			log.Println("Cannot retrieve reaction informations", err)
			return
		}
		for _, u := range users {
			//If the user has other votes, we remove them
			if u.ID == reactionAdd.UserID && r.Emoji.Name != reactionAdd.Emoji.Name && reactionIsOption(poll.Options, reactionAdd.Emoji.Name) {
				session.MessageReactionRemove(channel.ChannelUid, poll.MessageUid, r.Emoji.Name, u.ID)
				break
			}
		}
	}
}

func reactionIsOption(options []*db.PollOption, emojiID string) bool {
	for _, o := range options {
		if o.ReactionId == emojiID {
			return true
		}
	}
	return false
}