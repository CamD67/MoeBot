package db

import (
	"database/sql"
	"log"
	"strconv"
	"strings"

	"github.com/camd67/moebot/moebot_bot/util/db/types"
)

const (
	roleGroupTable = `CREATE TABLE IF NOT EXISTS role_group(
			Id SERIAL NOT NULL PRIMARY KEY,
			ServerId INTEGER NOT NULL REFERENCES Server(Id),
			Name TEXT NOT NULL CHECK(char_length(Name) <= 500),
			Type INTEGER NOT NULL
		)`

	RoleGroupMaxNameLength       = 500
	RoleGroupMaxNameLengthString = "500"

	roleGroupQueryById     = `SELECT Id, ServerId, Name, Type FROM role_group WHERE Id = $1`
	roleGroupQueryByName   = `SELECT rg.Id, rg.ServerId, rg.Name, rg.Type FROM role_group AS rg WHERE rg.Name = $1 AND rg.ServerId = $2`
	roleGroupQueryByServer = `SELECT Id, ServerId, Name, Type FROM role_group WHERE ServerId = $1`
	roleGroupInsert        = `INSERT INTO role_group(ServerId, Name, Type) VALUES ($1, $2, $3) RETURNING Id`
	roleGroupUpdate        = `UPDATE role_group SET Name = $2, Type = $3 WHERE Id = $1`
	roleGroupDeleteId      = `DELETE FROM role_group WHERE Id = $1`

	UncategorizedGroup = "Uncategorized"
)

func RoleGroupInsertOrUpdate(rg types.RoleGroup, s types.Server) (newId int, err error) {
	row := moeDb.QueryRow(roleGroupQueryById, rg.Id)
	var dbRg types.RoleGroup
	if err := row.Scan(&dbRg.Id, &dbRg.ServerId, &dbRg.Name, &dbRg.Type); err != nil {
		if err == sql.ErrNoRows {
			// no row, so insert it add in default values
			if rg.Type <= 0 {
				rg.Type = types.GroupTypeAny
			}
			err := moeDb.QueryRow(roleGroupInsert, s.Id, rg.Name, rg.Type).Scan(&newId)
			if err != nil {
				log.Println("Error inserting roleGroup to db")
				return -1, err
			}

		} else {
			// got some other kind of error
			log.Println("Error scanning roleGroup row from database", err)
			return -1, err
		}
	} else {
		// got a row, update it
		if rg.Type > 0 {
			dbRg.Type = rg.Type
		}
		if rg.Name != "" {
			dbRg.Name = rg.Name
		}
		_, err = moeDb.Exec(roleGroupUpdate, dbRg.Id, dbRg.Name, dbRg.Type)
		if err != nil {
			log.Println("Error updating roleGroup to db: Id - " + strconv.Itoa(dbRg.Id))
			return -1, err
		}
		newId = dbRg.Id
	}
	return newId, nil
}

/*
Returns a RoleGroup matching the id inside the given RoleGroup. If no match is found, the RoleGroup is added to the database
*/
func RoleGroupQueryOrInsert(rg types.RoleGroup, s types.Server) (newRg types.RoleGroup, err error) {
	row := moeDb.QueryRow(roleGroupQueryById, rg.Id)
	if err = row.Scan(&newRg.Id, &newRg.ServerId, &newRg.Name, &newRg.Type); err != nil {
		if err == sql.ErrNoRows {
			// no row, so insert it add in default values
			if rg.Type <= 0 {
				rg.Type = types.GroupTypeAny
			}
			var insertId int
			err = moeDb.QueryRow(roleGroupInsert, s.Id, rg.Name, rg.Type).Scan(&insertId)
			if err != nil {
				log.Println("Error inserting role to db")
				return
			}
			// no need to re-query since we inserted a row
			newRg.Id = insertId
		} else {
			log.Println("Error scanning in roleGroup", err)
			return types.RoleGroup{}, err
		}
	}
	return
}

func RoleGroupQueryServer(s types.Server) (roleGroups []types.RoleGroup, err error) {
	rows, err := moeDb.Query(roleGroupQueryByServer, s.Id)
	if err != nil {
		log.Println("Error querying for roleGroup", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var rg types.RoleGroup
		if err = rows.Scan(&rg.Id, &rg.ServerId, &rg.Name, &rg.Type); err != nil {
			log.Println("Error scanning from roleGroup table:", err)
			return
		}
		roleGroups = append(roleGroups, rg)
	}
	return
}

func RoleGroupQueryName(name string, serverId int) (rg types.RoleGroup, err error) {
	row := moeDb.QueryRow(roleGroupQueryByName, name, serverId)
	err = row.Scan(&rg.Id, &rg.ServerId, &rg.Name, &rg.Type)
	if err != nil && err != sql.ErrNoRows {
		log.Println("Error querying for role group by name and serverID", err)
	}
	// return whatever we get, error or row
	return
}

func RoleGroupQueryId(id int) (rg types.RoleGroup, err error) {
	row := moeDb.QueryRow(roleGroupQueryById, id)
	err = row.Scan(&rg.Id, &rg.ServerId, &rg.Name, &rg.Type)
	if err != nil && err != sql.ErrNoRows {
		log.Println("Error querying for role group by id", err)
	}
	return
}

func RoleGroupDelete(id int) error {
	_, err := moeDb.Exec(roleGroupDeleteId, id)
	if err != nil && err != sql.ErrNoRows {
		log.Println("Error deleting role group: ", id)
	}
	return err
}

func roleGroupCreateTable() {
	_, err := moeDb.Exec(roleGroupTable)
	if err != nil {
		log.Println("Error creating role group table", err)
		return
	}
	//for _, alter := range roleUpdateTable {
	//	_, err = moeDb.Exec(alter)
	//	if err != nil {
	//		log.Println("Error alterting role group table", err)
	//		return
	//	}
	//}
}

func GetGroupTypeFromString(s string) types.GroupType {
	toCheck := strings.ToUpper(s)
	if toCheck == "ANY" {
		return types.GroupTypeAny
	} else if toCheck == "EXCLUSIVE" || toCheck == "EXC" {
		return types.GroupTypeExclusive
	} else if toCheck == "EXCLUSIVE NO REMOVE" || toCheck == "ENR" {
		return types.GroupTypeExclusiveNoRemove
	} else if toCheck == "NO MULTIPLES" || toCheck == "NOM" {
		return types.GroupTypeNoMultiples
	} else {
		return -1
	}
}

func GetStringFromGroupType(groupType types.GroupType) string {
	switch groupType {
	case types.GroupTypeAny:
		return "Any (ANY)"
	case types.GroupTypeExclusive:
		return "Exclusive (EXC)"
	case types.GroupTypeExclusiveNoRemove:
		return "Exclusive No Remove (ENR)"
	case types.GroupTypeNoMultiples:
		return "No Multiples (NOM)"
	default:
		return "Unknown"
	}
}
