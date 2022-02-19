package main

import (
	"database/sql"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func getGiveawayForGuild(guildId string) *Giveaway {
	var giveaway Giveaway
	err := DbMap.SelectOne(&giveaway, "SELECT * FROM Giveaways WHERE guild_id = ? AND end_time IS NULL", guildId)
	if err != nil && err != sql.ErrNoRows {
		log.Panicln("getGiveawayForGuild#DbMap.SelectOne", err)
	}
	if err == sql.ErrNoRows {
		return nil
	}
	return &giveaway
}

func getAllUnfinishedGiveaways() []Giveaway {
	var res []Giveaway
	_, err := DbMap.Select(&res, "SELECT * FROM Giveaways WHERE end_time IS NULL")
	if err != nil {
		log.Panicln("getAllUnfinishedGiveaways#DbMap.Select", err)
		return nil
	}
	return res
}

func createMissingGiveaways(guild *discordgo.Guild) {
	_, err := session.Channel(getGiveawayChannelIdForGuild(guild.ID))
	if err == nil {
		giveaway := getGiveawayForGuild(guild.ID)
		if giveaway == nil {
			giveaway = &Giveaway{
				StartTime: time.Now(),
				GuildId:   guild.ID,
				GuildName: guild.Name,
			}
			err := DbMap.Insert(giveaway)
			if err != nil {
				log.Panicln("createMissingGiveaways#DbMap.Insert", err)
				return
			}
		}

	}
}

func getGiveawayChannelIdForGuild(guildId string) string {
	var serverConfig ServerConfig
	err := DbMap.SelectOne(&serverConfig, "SELECT * FROM ServerConfig WHERE guild_id = ?", guildId)
	if err != nil {
		log.Println("("+guildId+") getGiveawayChannelIdForGuild#DbMap.SelectOne", err)
		return ""
	}
	return serverConfig.MainChannel
}

func finishGiveaways() {
	giveaways := getAllUnfinishedGiveaways()
	for _, giveaway := range giveaways {
		finishGiveaway(giveaway.GuildId)
		guild, err := session.Guild(giveaway.GuildId)
		if err == nil {
			createMissingGiveaways(guild)
		}
	}
}

func finishGiveaway(guildId string) {
	giveaway := getGiveawayForGuild(guildId)
	if giveaway == nil {
		log.Println("(" + guildId + ") finishGiveaway#getGiveawayForGuild")
		return
	}
	_, err := session.Guild(giveaway.GuildId)
	if err != nil {
		log.Println("("+guildId+") finishGiveaway#session.Guild", err)
		return
	}
	giveawayChannelId := getGiveawayChannelIdForGuild(guildId)
	var participants []Participant
	_, err = DbMap.Select(&participants, "SELECT * FROM Participants WHERE giveaway_id = ? AND is_accepted = true", giveaway.Id)
	if err != nil {
		log.Panicln("("+guildId+") finishGiveaway#DbMap.Select", err)
	}
	if participants == nil || len(participants) == 0 {
		giveaway.EndTime.Time = time.Now()
		giveaway.EndTime.Valid = true
		_, err := DbMap.Update(giveaway)
		if err != nil {
			log.Panicln("("+guildId+") finishGiveaway#DbMap.Select", err)
		}
		notifyWinner(giveaway.GuildId, giveawayChannelId, nil, "")
		return
	}
	code, err := getCSRVCode()
	if err != nil {
		log.Println("("+guildId+") finishGiveaway#getCSRVCode", err)
		_, err = session.ChannelMessageSend(giveawayChannelId, "Błąd API Craftserve, nie udało się pobrać kodu!")
		if err != nil {
			return
		}
		return
	}
	rand.Seed(time.Now().UnixNano())
	winner := participants[rand.Int()%len(participants)]
	giveaway.InfoMessageId.String = notifyWinner(giveaway.GuildId, giveawayChannelId, &winner.UserId, code)
	giveaway.InfoMessageId.Valid = true
	giveaway.EndTime.Time = time.Now()
	giveaway.EndTime.Valid = true
	giveaway.Code.String = code
	giveaway.Code.Valid = true
	giveaway.WinnerId.String = winner.UserId
	giveaway.WinnerId.Valid = true
	giveaway.WinnerName.String = winner.UserName
	giveaway.WinnerName.Valid = true
	_, err = DbMap.Update(giveaway)
	if err != nil {
		log.Panicln("("+guildId+") finishGiveaway#DbMap.Update", err)
	}
}

func getParticipantsNames(giveawayId int) ([]string, error) {
	var participants []Participant
	_, err := DbMap.Select(&participants, "SELECT user_name FROM Participants WHERE giveaway_id = ? AND is_accepted = true", giveawayId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		log.Println("getParticipantsNames#DbMap.Select", err)
		return nil, err
	}
	names := make([]string, len(participants))
	for i := range participants {
		names[i] = participants[i].UserName
	}
	return names, nil
}

func getParticipantByMessageId(messageId string) *Participant {
	var participant Participant
	err := DbMap.SelectOne(&participant, "SELECT * FROM Participants WHERE message_id = ?", messageId)
	if err != nil && err != sql.ErrNoRows {
		log.Println("getParticipantByMessageId#DbMap.SelectOne", err)
		return nil
	}
	if err == sql.ErrNoRows {
		return nil
	}
	return &participant
}

func getParticipantsByGiveawayId(giveawayId int) []Participant {
	var participants []Participant
	_, err := DbMap.Select(&participants, "SELECT * FROM Participants WHERE giveaway_id = ? AND is_accepted = true", giveawayId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		log.Println("getParticipantsByGiveawayId#DbMap.Select", err)
		return nil
	}
	return participants
}

func getParticipantsNamesString(giveawayId int) (string, error) {
	participants, err := getParticipantsNames(giveawayId)
	if err != nil {
		return "", err
	}
	return strings.Join(participants, ", "), nil
}

func getParticipantCandidateByMessageId(messageId string) *ParticipantCandidate {
	var candidate ParticipantCandidate
	err := DbMap.SelectOne(&candidate, "SELECT * From ParticipantCandidates WHERE message_id = ?", messageId)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}

		log.Panicln("getParticipantCandidateByMessageId#DbMap.Select", err)
	}

	return &candidate
}

func notifyWinner(guildId, channelId string, winnerId *string, code string) string {
	if winnerId == nil {
		log.Println("(" + guildId + ") Giveaway ended without any participants.")
		message, err := session.ChannelMessageSend(channelId, "Dzisiaj nikt nie wygrywa, ponieważ nikt nie był w loterii.")
		if err != nil {
			return ""
		}
		return message.ID
	}

	winner, err := session.GuildMember(guildId, *winnerId)
	if err != nil {
		log.Println("("+guildId+") notifyWinner#session.GuildMember("+*winnerId+")", err)
		return ""
	}

	log.Println("(" + guildId + ") " + winner.User.Username + " has won the giveaway (Code: " + code + ").")
	embed := discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			URL:     "https://craftserve.pl",
			Name:    "Wygrałeś kod na serwer diamond!",
			IconURL: "https://cdn.discordapp.com/avatars/524308413719642118/c2a17b4479bfcc89d2b7e64e6ae15ebe.webp",
		},
		Description: "Gratulacje! W loterii wygrałeś darmowy kod na serwer w CraftServe! Możesz go użyć w zakładce *Płatności* pod przyciskiem *Zrealizuj kod podarunkowy*. Kod jest ważny około rok.",
	}
	embed.Fields = []*discordgo.MessageEmbedField{}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "KOD", Value: code})
	dm, err := session.UserChannelCreate(*winnerId)
	if err != nil {
		log.Println("("+guildId+") notifyWinner#session.UserChannelCreate", err)
	}
	_, err = session.ChannelMessageSendEmbed(dm.ID, &embed)
	embed = discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			URL:     "https://craftserve.pl",
			Name:    "Wyniki giveaway!",
			IconURL: "https://cdn.discordapp.com/avatars/524308413719642118/c2a17b4479bfcc89d2b7e64e6ae15ebe.webp",
		},
		Description: winner.User.Username + " wygrał kod. Gratulacje!",
	}
	message, err := session.ChannelMessageSendEmbed(channelId, &embed)
	if err != nil {
		log.Println("("+guildId+") notifyWinner#session.ChannelMessageSendEmbed", err)
		return ""
	}
	return message.ID
}

func deleteFromGiveaway(guildId, userId string) {
	giveaway := getGiveawayForGuild(guildId)
	if giveaway == nil {
		log.Println("(" + guildId + ") notifyWinner#getGiveawayForGuild")
		return
	}
	participants := getParticipantsByGiveawayId(giveaway.Id)
	for _, participant := range participants {
		if participant.UserId == userId {
			participant.IsAccepted.Valid = true
			participant.IsAccepted.Bool = false
			_, err := DbMap.Update(&participant)
			if err != nil {
				log.Panicln(err)
			}
		}
	}
	for _, participant := range participants {
		updateThxInfoMessage(&participant.MessageId, "", participant.ChannelId, participant.UserId, participant.GiveawayId, nil, reject)
	}
	return
}

func blacklistUser(guildId, userId, blacklisterId string) error {
	blacklist := &Blacklist{GuildId: guildId,
		UserId:        userId,
		BlacklisterId: blacklisterId}
	err := DbMap.Insert(blacklist)
	if err != nil {
		log.Println("("+guildId+") blacklistUser#DbMap.Insert", err)
		return err
	}
	return err
}

func unblacklistUser(guildId, userId string) error {
	_, err := DbMap.Exec("DELETE FROM Blacklists WHERE guild_id = ? AND user_id = ?", guildId, userId)
	if err != nil {
		log.Println("("+guildId+") unblacklistUser#DbMap.Exec", err)
		return err
	}
	return err
}

func helperBlacklistUser(guildId, userId, blacklisterId string) error {
	blacklist := &HelperBlacklist{GuildId: guildId,
		UserId:        userId,
		BlacklisterId: blacklisterId}
	err := DbMap.Insert(blacklist)
	if err != nil {
		log.Println("("+guildId+") helperBlacklistUser#DbMap.Insert", err)
		return err
	}
	return err
}

func checkHelperBlacklist(guildId string, userId string) bool {
	var helperBlacklist HelperBlacklist
	err := DbMap.SelectOne(&helperBlacklist, "SELECT * FROM HelperBlacklist WHERE user_id = ? AND guild_id = ?", userId, guildId)

	if err == sql.ErrNoRows {
		return false
	}

	if err != nil {
		log.Panicln("("+userId+") checkHelperBlacklist#DbMap.SelectOne", err)
		return true
	}
	return true
}

func helperUnBlacklistUser(guildId, userId string) error {
	_, err := DbMap.Exec("DELETE FROM HelperBlacklist WHERE guild_id = ? AND user_id = ?", guildId, userId)
	if err != nil {
		log.Println("("+guildId+") helperUnBlacklistUser#DbMap.Exec", err)
		return err
	}
	return err
}

func isBlacklisted(guildId, userId string) bool {
	ret, err := DbMap.SelectInt("SELECT count(*) FROM Blacklists WHERE guild_id = ? AND user_id = ?", guildId, userId)
	if err != nil {
		log.Println("("+guildId+") isBlacklisted#DbMap.SelectInt", err)
		return false
	}
	if ret == 1 {
		return true
	}
	return false
}
