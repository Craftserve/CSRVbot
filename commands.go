package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/imroc/req"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func handleThxCommand(m *discordgo.MessageCreate, args []string) {
	if len(args) != 2 {
		printGiveawayInfo(m.ChannelID, m.GuildID)
		return
	}

	match, err := regexp.Match("<@[!]?[0-9]*>", []byte(args[1]))
	if err != nil {
		log.Println("("+m.GuildID+") handleThxCommand#regexp.Match", err)
		return
	}

	if !match {
		printGiveawayInfo(m.ChannelID, m.GuildID)
		return
	}

	args[1] = args[1][2 : len(args[1])-1]
	if strings.HasPrefix(args[1], "!") {
		args[1] = args[1][1:]
	}
	if m.Author.ID == args[1] {
		_, err := session.ChannelMessageSend(m.ChannelID, "Nie można dziękować sobie!")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}
	user, err := session.User(args[1])
	if err != nil {
		log.Println("("+m.GuildID+") handleThxCommand#session.User", err)
		return
	}
	guild, err := session.Guild(m.GuildID)
	if err != nil {
		_, err = session.ChannelMessageSend(m.ChannelID, "Coś poszło nie tak przy dodawaniu podziękowania :(")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		log.Println("("+m.GuildID+") handleThxCommand#session.Guild", err)
		return
	}
	log.Println("(" + m.GuildID + ") " + m.Author.Username + " has thanked " + user.Username)
	if user.Bot {
		_, err = session.ChannelMessageSend(m.ChannelID, "Nie można dziękować botom!")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}
	if isBlacklisted(m.GuildID, m.Mentions[0].ID) {
		_, err = session.ChannelMessageSend(m.ChannelID, "Ten użytkownik jest na czarnej liście i nie może brać udziału :(")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}
	giveaway := getGiveawayForGuild(m.GuildID)
	if giveaway == nil {
		log.Println("(" + m.GuildID + ") handleThxCommand#getGiveawayForGuild")
		return
	}
	participant := &Participant{
		UserId:     args[1],
		GiveawayId: giveaway.Id,
		CreateTime: time.Now(),
		GuildId:    m.GuildID,
		ChannelId:  m.ChannelID,
	}
	participant.GuildName = guild.Name
	participant.UserName = user.Username
	participant.MessageId = *updateThxInfoMessage(nil, m.GuildID, m.ChannelID, args[1], participant.GiveawayId, nil, wait)
	err = DbMap.Insert(participant)
	if err != nil {
		_, err = session.ChannelMessageSend(m.ChannelID, "Coś poszło nie tak przy dodawaniu podziękowania :(")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		log.Panicln("("+m.GuildID+") handleThxCommand#DbMap.Insert", err)
	}
	for err = session.MessageReactionAdd(m.ChannelID, participant.MessageId, "✅"); err != nil; err = session.MessageReactionAdd(m.ChannelID, participant.MessageId, "✅") {
	}
	for err = session.MessageReactionAdd(m.ChannelID, participant.MessageId, "⛔"); err != nil; err = session.MessageReactionAdd(m.ChannelID, participant.MessageId, "⛔") {
	}
}

func handleCsrvbotCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) >= 2 {
		switch args[1] {
		case "start":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("OnMessageCreate s.GuildMember(" + m.GuildID + ", " + m.Message.Author.ID + ") " + err.Error())
				return
			}
			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}
			guild, err := s.Guild(m.GuildID)
			if err != nil {
				log.Println("OnMessageCreate s.Guild(" + m.GuildID + ")")
				guild, err = s.Guild(m.GuildID)
				if err != nil {
					return
				}
			}
			finishGiveaway(m.GuildID)
			createMissingGiveaways(guild)
			return
		case "delete":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("("+m.GuildID+") handleCsrvbotCommand#s.GuildMember("+m.Message.Author.ID+")", err)
				return
			}

			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			if len(args) == 2 {
				_, err := s.ChannelMessageSend(m.ChannelID, "Musisz podać ID użytkownika!")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			guild, err := session.Guild(m.GuildID)
			if len(m.Mentions) < 1 {
				if err != nil {
					log.Println(m.Author.Username + " usunął ID " + args[2] + " z giveawaya na " + m.GuildID)
					log.Println(err)
					return
				}

				log.Println(m.Author.Username + " usunął ID " + args[2] + " z giveawaya na " + guild.Name)
				deleteFromGiveaway(m.GuildID, args[2])

				_, err = s.ChannelMessageSend(m.ChannelID, "Usunięto z giveawaya.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			if err != nil {
				log.Println(m.Author.Username + " usunął " + m.Mentions[0].Username + " z giveawaya na " + m.GuildID)
				log.Println(err)
				return
			}

			log.Println(m.Author.Username + " usunął " + m.Mentions[0].Username + " z giveawaya na " + guild.Name)
			deleteFromGiveaway(m.GuildID, m.Mentions[0].ID)

			_, err = s.ChannelMessageSend(m.ChannelID, "Usunięto z giveawaya.")
			if err != nil {
				log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
				return
			}
			return
		case "blacklist":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("("+m.GuildID+") handleCsrvbotCommand#s.GuildMember("+m.Message.Author.ID+")", err)
				return
			}

			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			if len(args) == 2 {
				_, err = s.ChannelMessageSend(m.ChannelID, "Musisz podać użytkownika!")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			guild, err := session.Guild(m.GuildID)
			if len(m.Mentions) < 1 {
				if err != nil {
					log.Println(m.Author.Username + " zblacklistował ID " + args[2] + " na " + m.GuildID)
					log.Println("("+m.GuildID+") handleCsrvbotCommand#session.Guild", err)
					return
				}
				log.Println(m.Author.Username + " zblacklistował ID " + args[2] + " na " + guild.Name)
				if blacklistUser(m.GuildID, args[2], m.Author.ID) == nil {
					_, err = s.ChannelMessageSend(m.ChannelID, "Użytkownik został zablokowany z możliwości udziału w giveaway.")
					if err != nil {
						log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
						return
					}
				}
				return
			}

			if err != nil {
				log.Println(m.Author.Username + " zblacklistował " + m.Mentions[0].Username + " na " + m.GuildID)
				log.Println("("+m.GuildID+") handleCsrvbotCommand#session.Guild", err)
				return
			}

			log.Println(m.Author.Username + " zblacklistował " + m.Mentions[0].Username + " na " + guild.Name)
			if blacklistUser(m.GuildID, m.Mentions[0].ID, m.Author.ID) == nil {
				_, err = s.ChannelMessageSend(m.ChannelID, "Użytkownik został zablokowany z możliwości udziału w giveaway.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
			}

			return
		case "unblacklist":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("("+m.GuildID+") handleCsrvbotCommand#s.GuildMember("+m.Message.Author.ID+")", err)
				return
			}

			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			if len(args) == 2 {
				_, err = s.ChannelMessageSend(m.ChannelID, "Musisz podać użytkownika!")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			guild, err := session.Guild(m.GuildID)
			if len(m.Mentions) < 1 {
				if err != nil {
					log.Println(m.Author.Username + " odblacklistował ID " + args[2] + " na " + m.GuildID)
					log.Println("("+m.GuildID+") handleCsrvbotCommand#session.Guild", err)
					return
				}

				log.Println(m.Author.Username + " odblacklistował ID " + args[2] + " na " + guild.Name)
				if unblacklistUser(m.GuildID, args[2]) == nil {
					_, err = s.ChannelMessageSend(m.ChannelID, "Użytkownik ponownie może brać udział w giveawayach.")
					if err != nil {
						log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
						return
					}
				}
				return
			}
			if err != nil {
				log.Println(m.Author.Username + " odblacklistował " + m.Mentions[0].Username + " na " + m.GuildID)
				log.Println("("+m.GuildID+") handleThxCommand#session.Guild", err)
				return
			}
			log.Println(m.Author.Username + " odblacklistował " + m.Mentions[0].Username + " na " + guild.Name)
			if unblacklistUser(m.GuildID, m.Mentions[0].ID) == nil {
				_, err = s.ChannelMessageSend(m.ChannelID, "Użytkownik ponownie może brać udział w giveawayach.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
			}
			return
		case "setGiveawayChannel":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("("+m.GuildID+") handleThxCommand#s.GuildMember("+m.Message.Author.ID+")", err)
				return
			}

			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			if len(args) == 2 {
				_, err := s.ChannelMessageSend(m.ChannelID, "Musisz podać kanał!")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			serverConfig := getServerConfigForGuildId(m.GuildID)
			if strings.HasPrefix(args[2], "<#") {
				args[2] = args[2][2:]
				args[2] = args[2][:len(args[2])-1]
			}

			serverConfig.MainChannel = args[2]
			_, err = DbMap.Update(&serverConfig)
			if err != nil {
				log.Panic("("+m.GuildID+") handleThxCommand#DbMap.Update(&serverConfig)", err)
			}

			_, err = s.ChannelMessageSend(m.ChannelID, "Ustawiono.")
			if err != nil {
				log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
				return
			}
			return
		case "setBotAdminRoleName":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("("+m.GuildID+") handleThxCommand#s.GuildMember("+m.Message.Author.ID+")", err)
				return
			}

			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			if len(args) == 2 {
				_, err := s.ChannelMessageSend(m.ChannelID, "Musisz podać nazwę roli!")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			serverConfig := getServerConfigForGuildId(m.GuildID)
			serverConfig.AdminRole = args[2]

			_, err = DbMap.Update(&serverConfig)
			if err != nil {
				log.Panic("("+m.GuildID+") handleThxCommand#DbMap.Update(&serverConfig)", err)
			}

			_, err = s.ChannelMessageSend(m.ChannelID, "Ustawiono.")
			if err != nil {
				log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
				return
			}
			return
		case "setThxInfoChannel":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("("+m.GuildID+") handleThxCommand#s.GuildMember("+m.Message.Author.ID+")", err)
				return
			}

			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			if len(args) == 2 {
				_, err := s.ChannelMessageSend(m.ChannelID, "Musisz podać kanał!")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			serverConfig := getServerConfigForGuildId(m.GuildID)
			if strings.HasPrefix(args[2], "<#") {
				args[2] = args[2][2:]
				args[2] = args[2][:len(args[2])-1]
			}

			serverConfig.ThxInfoChannel = args[2]

			_, err = DbMap.Update(&serverConfig)
			if err != nil {
				log.Panic("("+m.GuildID+") handleThxCommand#DbMap.Update(&serverConfig)", err)
			}

			_, err = s.ChannelMessageSend(m.ChannelID, "Ustawiono.")
			if err != nil {
				log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
				return
			}

			return
		case "resend":
			embed, err := generateResendEmbed(m.Message.Author.ID)
			if err != nil {
				log.Println("handleCsrvbotCommand#generateResendEmbed(" + m.Message.Author.ID + ") " + err.Error())
			}

			dm, err := session.UserChannelCreate(m.Message.Author.ID)
			if err != nil {
				log.Println("handleCsrvbotCommand#UserChannelCreate", err)
				return
			}

			_, err = session.ChannelMessageSendEmbed(dm.ID, embed)
			if err != nil {
				_, err := session.ChannelMessageSend(m.ChannelID, "Nie udało się wysłać kodów, ponieważ masz wyłączone wiadomości prywatne.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			_, err = s.ChannelMessageSend(m.ChannelID, "Kody zostały ponownie wysłane.")
			if err != nil {
				log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
				return
			}

			log.Println("Wysłano resend do " + m.Author.Username + "#" + m.Author.Discriminator)
			return
		case "setHelperRoleName":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("("+m.GuildID+") handleThxCommand#s.GuildMember("+m.Message.Author.ID+")", err)
				return
			}

			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			if len(args) == 2 {
				_, err := s.ChannelMessageSend(m.ChannelID, "Musisz podać nazwę roli!")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}

			serverConfig := getServerConfigForGuildId(m.GuildID)
			serverConfig.HelperRoleName = args[2]

			_, err = DbMap.Update(&serverConfig)
			if err != nil {
				log.Panic("("+m.GuildID+") handleThxCommand#DbMap.Update(&serverConfig)", err)
			}

			_, err = s.ChannelMessageSend(m.ChannelID, "Ustawiono.")
			if err != nil {
				log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
				return
			}

			return
		case "setHelperRoleNeededThxAmount":
			member, err := s.GuildMember(m.GuildID, m.Message.Author.ID)
			if err != nil {
				log.Println("("+m.GuildID+") handleThxCommand#s.GuildMember("+m.Message.Author.ID+")", err)
				return
			}
			if !hasAdminPermissions(member, m.GuildID) {
				_, err = s.ChannelMessageSend(m.ChannelID, "Brak uprawnień.")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}
			if len(args) == 2 {
				_, err := s.ChannelMessageSend(m.ChannelID, "Musisz podać nazwę roli!")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}
			serverConfig := getServerConfigForGuildId(m.GuildID)
			num, err := strconv.Atoi(args[2])
			if err != nil {
				_, err = s.ChannelMessageSend(m.ChannelID, "Ale moze liczbe daj co")
				if err != nil {
					log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
					return
				}
				return
			}
			serverConfig.HelperRoleThxesNeeded = num
			_, err = DbMap.Update(&serverConfig)
			if err != nil {
				log.Panic("OnMessageCreate DbMap.Update(&serverConfig) " + err.Error())
			}

			_, err = s.ChannelMessageSend(m.ChannelID, "Ustawiono.")
			if err != nil {
				log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
				return
			}

			checkHelpers(m.GuildID)
			return
		}

	}
	_, err := s.ChannelMessageSend(m.ChannelID, "!csrvbot <`start` | `delete` | `resend` | `blacklist` | `unblacklist` | `setGiveawayChannel` | `setBotAdminRoleName` | `setThxInfoChannel` | `setHelperRoleName` | `setHelperRoleNeededThxAmount`>")
	if err != nil {
		log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
		return
	}
}

func handleThxmeCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) != 2 {
		_, err := s.ChannelMessageSend(m.ChannelID, "Niepoprawna ilość argumentów")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}

	match, err := regexp.Match("<@[!]?[0-9]*>", []byte(args[1]))
	if err != nil {
		log.Println("("+m.GuildID+") handleThxmeCommand#regexp.Match", err)
		return
	}

	if !match {
		printGiveawayInfo(m.ChannelID, m.GuildID)
		return
	}
	args[1] = args[1][2 : len(args[1])-1]
	if strings.HasPrefix(args[1], "!") {
		args[1] = args[1][1:]
	}
	if m.Author.ID == args[1] {
		_, err := session.ChannelMessageSend(m.ChannelID, "Nie można poprosić o podziękowanie samego siebie!")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}

	user, err := session.User(args[1])
	if err != nil {
		log.Println("handleThxmeCommand#session.User", err)
		return
	}

	guild, err := session.Guild(m.GuildID)
	if err != nil {
		_, err = session.ChannelMessageSend(m.ChannelID, "Coś poszło nie tak przy dodawaniu podziękowania :(")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		log.Println("("+guild.ID+") handleThxmeCommand#session.Guild", err)
		return
	}
	log.Println("(" + guild.ID + ") " + m.Author.Username + " has thanked " + user.Username)
	if user.Bot {
		_, err = session.ChannelMessageSend(m.ChannelID, "Nie można prosić o podziękowanie bota!")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}
	if isBlacklisted(m.GuildID, m.Author.ID) {
		_, err = session.ChannelMessageSend(m.ChannelID, "Nie możesz poprosić o podziękowanie, gdyż jesteś na czarnej liście!")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}
	candidate := &ParticipantCandidate{
		CandidateName:         m.Author.Username,
		CandidateId:           m.Author.ID,
		CandidateApproverName: user.Username,
		CandidateApproverId:   user.ID,
		GuildName:             guild.Name,
		GuildId:               m.GuildID,
		ChannelId:             m.ChannelID,
	}
	messageId, err := s.ChannelMessageSend(m.ChannelID, user.Mention()+", czy chcesz podziękować użytkownikowi "+m.Author.Mention()+"?")
	if err != nil {
		_, err = session.ChannelMessageSend(m.ChannelID, "Coś poszło nie tak przy dodawaniu kandydata do podziekowania :(")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		log.Panicln("("+guild.ID+") handleThxmeCommand#session.ChannelMessageSend", err)
	}
	candidate.MessageId = messageId.ID
	err = DbMap.Insert(candidate)
	if err != nil {
		_, err = session.ChannelMessageSend(m.ChannelID, "Coś poszło nie tak przy dodawaniu kandydata do podziękowania :(")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		log.Println("("+guild.ID+") handleThxmeCommand#DbMap.Insert", err)
		return
	}
	for err = session.MessageReactionAdd(m.ChannelID, candidate.MessageId, "✅"); err != nil; err = session.MessageReactionAdd(m.ChannelID, candidate.MessageId, "✅") {
	}
	for err = session.MessageReactionAdd(m.ChannelID, candidate.MessageId, "⛔"); err != nil; err = session.MessageReactionAdd(m.ChannelID, candidate.MessageId, "⛔") {
	}
}

func handleDocCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) == 1 {
		_, err := s.ChannelMessageSend(m.ChannelID, "Musisz podać jakiś poradnik.")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}
	docFile := strings.ToLower(args[1])

	// Hooking to Github API
	r, err := req.Get("https://api.github.com/repos/craftserve/docs/contents/" + docFile + ".md")
	if err != nil {
		log.Println("handleDocCommand Unable to hook into github api", err)
		return
	}

	// Checking if file exists
	resp := r.Response()
	if resp.StatusCode != 200 || (args[1] == "readme" || args[1] == "todo") {
		_, err = s.ChannelMessageSend(m.ChannelID, "Taki poradnik nie istnieje.")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}

	if len(args) == 2 {
		_, err := s.ChannelMessageSend(m.ChannelID, "<https://github.com/craftserve/docs/blob/master/"+docFile+".md>")
		if err != nil {
			log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
			return
		}
		return
	}

	anchor := strings.ReplaceAll(strings.Join(args[2:], "-"), "?", "")
	_, err = s.ChannelMessageSend(m.ChannelID, "<https://github.com/craftserve/docs/blob/master/"+docFile+".md#"+anchor+">")
	if err != nil {
		log.Println("("+m.GuildID+") Could not send message to channel ("+m.ChannelID+")", err)
		return
	}
	return
}
