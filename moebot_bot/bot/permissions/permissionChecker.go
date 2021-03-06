package permissions

import (
	"github.com/bwmarrin/discordgo"
	"github.com/camd67/moebot/moebot_bot/util/db"
	"github.com/camd67/moebot/moebot_bot/util/db/types"
)

type PermissionChecker struct {
	MasterId string
}

func (p *PermissionChecker) HasAllPerm(userId string, roles []string, guild *discordgo.Guild) bool {
	return p.HasPermission(userId, roles, guild, types.PermAll)
}

func (p *PermissionChecker) HasModPerm(userId string, roles []string, guild *discordgo.Guild) bool {
	return p.HasPermission(userId, roles, guild, types.PermMod)
}

func (p *PermissionChecker) HasPermission(userId string, roles []string, guild *discordgo.Guild, permToCheck types.Permission) bool {
	if permToCheck == types.PermAll {
		// if everyone can use this command, just allow it
		return true
	} else if p.IsMaster(userId) {
		// masters are allowed to do anything
		return true
	} else if IsGuildOwner(guild, userId) && permToCheck <= types.PermGuildOwner {
		// Special check for guild owners
		return true
	} else if permToCheck == types.PermNone {
		// if no one can use this command, never do it
		// make sure this is the last thing to check before
		return false
	}
	// if any of the previous checks fails, then go ahead and check the database for their permission
	perms := db.RoleQueryPermission(roles)
	for _, userPerm := range perms {
		if userPerm >= permToCheck {
			return true
		}
	}
	return false
}

func (p *PermissionChecker) IsMaster(id string) bool {
	return p.MasterId == id
}

func IsGuildOwner(guild *discordgo.Guild, id string) bool {
	return guild.OwnerID == id
}
