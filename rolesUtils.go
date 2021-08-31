package main

import (
	"errors"
	"log"

	"github.com/bwmarrin/discordgo"
)

func getRoleID(guildId string, roleName string) (string, error) {
	guild, err := session.Guild(guildId)
	if err != nil {
		log.Println("("+guildId+") "+"getRoleID#session.Guild", err)
		return "", err
	}

	roles := guild.Roles
	for _, role := range roles {
		if role.Name == roleName {
			return role.ID, nil
		}
	}

	return "", errors.New("no " + roleName + " role available")
}

func hasRole(member *discordgo.Member, roleName, guildId string) bool {
	adminRole, err := getRoleID(guildId, roleName)
	if err != nil {
		log.Println("("+guildId+") "+"hasRole#getRoleID("+roleName+")", err)
		return false
	}

	for _, role := range member.Roles {
		if role == adminRole {
			return true
		}
	}

	return false
}

func hasPermission(member *discordgo.Member, guildId string, permission int64) bool {
	for _, roleId := range member.Roles {
		role, err := session.State.Role(guildId, roleId)
		if err != nil {
			log.Println("("+guildId+") "+"hasPermisson#session.State.Role", err)
			return false
		}
		if role.Permissions&permission != 0 {
			return true
		}
	}
	return false
}

func hasAdminPermissions(member *discordgo.Member, guildId string) bool {
	if hasRole(member, getAdminRoleForGuild(guildId), guildId) || hasPermission(member, guildId, 8) {
		return true
	}

	return false
}

func getAdminRoleForGuild(guildId string) string {
	var serverConfig ServerConfig
	err := DbMap.SelectOne(&serverConfig, "SELECT * FROM ServerConfig WHERE guild_id = ?", guildId)
	if err != nil {
		log.Println("("+guildId+") "+"getAdminRoleForGuild#DbMap.SelectOne", err)
		return ""
	}

	return serverConfig.AdminRole
}

func getSavedRoles(guildId, memberId string) ([]string, error) {
	var memberRoles []MemberRole
	_, err := DbMap.Select(&memberRoles, "SELECT * FROM MemberRoles WHERE guild_id = ? AND member_id = ?", guildId, memberId)
	if err != nil {
		return nil, err
	}

	var ret []string
	for _, role := range memberRoles {
		ret = append(ret, role.RoleId)
	}

	return ret, nil
}

func updateMemberSavedRoles(member *discordgo.Member, guildId string) {
	savedRoles, err := getSavedRoles(guildId, member.User.ID)
	if err != nil {
		log.Println("("+guildId+") "+"updateMemberSavedRoles#getSavedRoles Error while getting saved roles", err)
		return
	}

	for _, memberRole := range member.Roles {
		found := false
		for i, savedRole := range savedRoles {
			if savedRole == memberRole {
				found = true
				savedRoles[i] = ""
				break
			}
		}
		if !found {
			role := MemberRole{GuildId: guildId, RoleId: memberRole, MemberId: member.User.ID}
			err = DbMap.Insert(&role)
			if err != nil {
				log.Println("("+guildId+") "+"updateAllMembersSavedRoles#DbMap.Insert Error while saving new role info", err)
				continue
			}
		}
	}

	for _, savedRole := range savedRoles {
		if savedRole != "" {
			_, err = DbMap.Exec("DELETE FROM MemberRoles WHERE guild_id = ? AND role_id = ? AND member_id = ?", guildId, savedRole, member.User.ID)
			if err != nil {
				log.Println("("+guildId+") "+"updateAllMembersSavedRoles#DbMap.Exec Error while deleting info about member role", err)
				continue
			}
		}
	}
}

func restoreMemberRoles(member *discordgo.Member, guildId string) {
	var memberRoles []MemberRole

	_, err := DbMap.Select(&memberRoles, "SELECT * FROM MemberRoles WHERE guild_id = ? AND member_id = ?", guildId, member.User.ID)
	if err != nil {
		log.Println("("+guildId+") "+"restoreMemberRoles#DbMap.Select Error while selecting roles", err)
		return
	}

	for _, role := range memberRoles {
		err = session.GuildMemberRoleAdd(guildId, member.User.ID, role.RoleId)
		if err != nil {
			log.Println("("+guildId+") "+"restoreMemberRoles#session.GuildMemberRoleAdd Error while adding role to member", err)
			continue
		}
	}
}

func updateAllMembersSavedRoles(guildId string) {
	guildMembers := getAllMembers(guildId)
	for _, member := range guildMembers {
		updateMemberSavedRoles(member, guildId)
	}
}
