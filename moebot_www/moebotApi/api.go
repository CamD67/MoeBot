package moebotApi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/bwmarrin/discordgo"

	botDb "github.com/camd67/moebot/moebot_bot/util/db"
	"github.com/camd67/moebot/moebot_www/auth"
	"github.com/camd67/moebot/moebot_www/db"
)

type MoebotApi struct {
	Amw      *auth.AuthenticationMiddleware
	Session  *discordgo.Session
	MoeWebDb *db.MoeWebDb
}

func (a *MoebotApi) UserInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := r.Header.Get("X-UserID")
	dbUser, err := a.MoeWebDb.Users.SelectByID(userID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(errorResponse("Cannot read user from Database"))
		return
	}
	var response []byte
	if dbUser.DiscordUID.Valid {
		discordUser, err := a.Session.User(dbUser.DiscordUID.String)
		if err != nil {
			log.Println("Failed to load Discord user - ", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(errorResponse("Cannot find matching Discord User"))
			return
		}
		w.WriteHeader(http.StatusOK)
		response, _ = json.Marshal(&struct {
			Username string `json:"username"`
			Avatar   string `json:"avatar"`
		}{Username: discordUser.Username, Avatar: fmt.Sprintf("https://cdn.discordapp.com/avatars/%v/%v.gif", discordUser.ID, discordUser.Avatar)})
	} else {
		w.WriteHeader(http.StatusOK)
		response, _ = json.Marshal(&struct {
			Username string `json:"username"`
			Avatar   string `json:"avatar"`
		}{Username: dbUser.Username, Avatar: "/static/defaultDiscordAvatar.png"})
	}
	w.Write(response)
}

func (a *MoebotApi) Heartbeat(w http.ResponseWriter, r *http.Request) {
	token, err := a.Amw.GetToken(r.Header.Get("Authorization"))
	if err != nil || token == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if time.Unix(token.Claims.(*auth.MoeCustomClaims).ExpiresAt, 0).Sub(time.Now()).Hours() < 24*7 {
		authType, _ := strconv.ParseInt(r.Header.Get("X-AuthType"), 10, 32)
		ss, err := a.Amw.CreateSignedToken(r.Header.Get("X-UserID"), auth.AuthType(authType))
		response, err := json.Marshal(struct{ Jwt string }{Jwt: ss})
		if err != nil {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)
	}
}

func (a *MoebotApi) ServerList(w http.ResponseWriter, r *http.Request) {
	type guildData struct {
		ServerUID  string `json:"id"`
		Icon       string `json:"icon"`
		Name       string `json:"name"`
		UserRights int    `json:"userRights"`
	}
	w.Header().Set("Content-Type", "application/json")
	userID := r.Header.Get("X-UserID")
	responseData := []*guildData{}
	var response []byte

	discordUser, err := a.getDiscordUser(userID)
	if err != nil {
		log.Println("Failed to load Discord user", err)
		w.WriteHeader(http.StatusNotFound)
		w.Write(errorResponse("Cannot find matching Discord User"))
		return
	}

	guilds, err := a.Session.UserGuilds(100, "", "")
	if err != nil {
		log.Println("Failed to load Guilds - ", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(errorResponse("Cannot find user guilds"))
		return
	}

	for _, g := range guilds {
		if m, err := a.Session.GuildMember(g.ID, discordUser.ID); err == nil {
			guild, err := a.Session.Guild(g.ID)
			if err == nil {
				m.GuildID = guild.ID
				var icon string
				if g.Icon != "" {
					icon = fmt.Sprintf("https://cdn.discordapp.com/icons/%v/%v.png", guild.ID, g.Icon)
				} else {
					icon = "/static/defaultDiscordAvatar.png"
				}
				responseData = append(responseData, &guildData{ServerUID: g.ID, Icon: icon, Name: g.Name, UserRights: int(getPermissionLevel(m, guild))})
			}
		}
	}
	response, _ = json.Marshal(responseData)
	w.WriteHeader(http.StatusOK)
	w.Write(response)
	return
}

func (a *MoebotApi) GetEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	isMod, err := a.checkGuildPermissions(r.Header.Get("X-UserID"), vars["GuildUID"], botDb.PermMod)
	if err != nil || !isMod {
		log.Println("Unknown user or no permissions to complete the request", err)
		w.WriteHeader(http.StatusForbidden)
		w.Write(errorResponse("Unable to complete the request"))
		return
	}
	from, err := time.Parse(time.RFC3339, vars["From"])
	if err != nil {
		log.Println("Cannot parse FROM date for request", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errorResponse("Invalid FROM date"))
		return
	}
	to, err := time.Parse(time.RFC3339, vars["To"])
	if err != nil {
		log.Println("Cannot parse TO date for request", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errorResponse("Invalid TO date"))
		return
	}

	events, err := a.MoeWebDb.GuildEvents.SelectEvents(from, to, vars["GuildUID"])
	response, _ := json.Marshal(events)
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func (a *MoebotApi) AddEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	isMod, err := a.checkGuildPermissions(r.Header.Get("X-UserID"), vars["GuildUID"], botDb.PermMod)
	if err != nil || !isMod {
		log.Println("Unknown user or no permissions to complete the request", err)
		w.WriteHeader(http.StatusForbidden)
		w.Write(errorResponse("Unable to complete the request"))
		return
	}

	event := &db.GuildEvent{}
	if err = json.NewDecoder(r.Body).Decode(event); err != nil {
		log.Println("Cannot parse request body", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errorResponse("Cannot parse the request"))
		return
	}

	event.GuildUID = vars["GuildUID"]
	event, err = a.MoeWebDb.GuildEvents.InsertEvent(event)
	if err != nil {
		log.Println("Cannot insert new event", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(errorResponse("Cannot insert new event"))
		return
	}

	response, _ := json.Marshal(event)
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func (a *MoebotApi) UpdateEvent(w http.ResponseWriter, r *http.Request) {

}

func (a *MoebotApi) DeleteEvent(w http.ResponseWriter, r *http.Request) {

}

func (a *MoebotApi) checkGuildPermissions(userID string, guildUID string, permLevel botDb.Permission) (bool, error) {
	isMoeGuild, err := a.checkMoeGuild(guildUID)
	if !isMoeGuild || err != nil {
		if err == nil {
			err = fmt.Errorf("No such guild in moebot database")
		}
		return false, err
	}
	discordUser, err := a.getDiscordUser(userID)
	if err != nil {
		return false, err
	}
	guild, err := a.Session.Guild(guildUID)
	if err != nil {
		log.Println("Cannot retreive guild - ", err)
		return false, err
	}
	member, err := a.Session.GuildMember(guildUID, discordUser.ID)
	if err != nil {
		log.Println("Cannot retreive guild member - ", err)
		return false, err
	}
	return getPermissionLevel(member, guild) >= permLevel, nil
}

func (a *MoebotApi) checkMoeGuild(guildUID string) (bool, error) {
	moeGuilds, err := a.Session.UserGuilds(100, "", "")
	if err != nil {
		log.Println("Cannot retreive moebot guilds - ", err)
		return false, err
	}
	for _, g := range moeGuilds {
		if g.ID == guildUID {
			return false, nil
		}
	}
	return false, nil
}

func (a *MoebotApi) getDiscordUser(userID string) (*discordgo.User, error) {
	dbUser, err := a.MoeWebDb.Users.SelectByID(userID)
	if err != nil {
		return nil, err
	}

	if !dbUser.DiscordUID.Valid {
		return nil, fmt.Errorf("No discord user informations available")
	}

	discordUser, err := a.Session.User(dbUser.DiscordUID.String)
	if err != nil {
		log.Println("Cannot retreive discord User - ", err)
		return nil, err
	}
	return discordUser, nil
}

func getPermissionLevel(member *discordgo.Member, guild *discordgo.Guild) botDb.Permission {
	if member.GuildID != guild.ID {
		return botDb.PermNone
	}
	if guild.OwnerID == member.User.ID {
		return botDb.PermGuildOwner
	}

	perms := botDb.RoleQueryPermission(member.Roles)
	highestPerm := botDb.PermAll
	// Find the highest permission level this user has
	for _, userPerm := range perms {
		if userPerm > highestPerm {
			highestPerm = userPerm
		}
	}
	return highestPerm
}

func errorResponse(errMessage string) []byte {
	resp, _ := json.Marshal(&struct {
		Error string `json:"error"`
	}{Error: errMessage})
	return resp
}
