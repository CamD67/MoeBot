package rolerules

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/camd67/moebot/moebot_bot/util/db"
	"github.com/camd67/moebot/moebot_bot/util/db/types"
)

//RoleActionType Type of action being performed on the role
type RoleActionType int

//RoleAction Action being performed on the role
type RoleAction struct {
	Role            *types.Role
	UserRank        *types.UserServerRank
	Member          *discordgo.Member
	Guild           *discordgo.Guild
	Channel         *discordgo.Channel
	OriginalMessage *discordgo.Message
	Action          RoleActionType
}

const (
	//RoleAdd Adding the role
	RoleAdd RoleActionType = 1
	//RoleRemove Removing the role
	RoleRemove RoleActionType = 2
)

//RoleRule Rule defining if and how a role should be applied
type RoleRule interface {
	Check(session *discordgo.Session, action *RoleAction) (success bool, message string)
	Apply(session *discordgo.Session, action *RoleAction) (success bool, message string)
}

func GetRulesForRole(server *types.Server, role *types.Role, comPrefix string) ([]RoleRule, error) {
	var result []RoleRule
	if server.VeteranRole.String == role.RoleUid && server.VeteranRank.Valid {
		result = append(result, &Points{PointsTreshold: int(server.VeteranRank.Int64)})
	}
	if role.ConfirmationMessage.Valid {
		result = append(result, &Confirmation{ComPrefix: comPrefix})
	}
	for _, gID := range role.Groups {
		group, err := db.RoleGroupQueryId(gID)
		if err != nil {
			log.Println("Error while retrieving role group during rules initialization", err)
			return nil, err
		}
		if group.Type == types.GroupTypeExclusive || group.Type == types.GroupTypeExclusiveNoRemove {
			relatedRoles, err := db.RoleQueryGroup(gID)
			if err != nil {
				log.Println("Error while retrieving related group roles during rules initialization", err)
				return nil, err
			}
			result = append(result, &Exclusive{ExclusiveRoles: relatedRoles})
		}
		if group.Type == types.GroupTypeExclusiveNoRemove {
			result = append(result, &NoRemove{ReferenceGroup: group})
		}
	}
	return result, nil
}
