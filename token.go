package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/rssnyder/discord-stock-ticker/utils"
)

type Token struct {
	Network   string        `json:"network"`
	Contract  string        `json:"contract"`
	Name      string        `json:"name"`
	Nickname  bool          `json:"nickname"`
	Frequency time.Duration `json:"frequency"`
	Color     bool          `json:"color"`
	Decorator string        `json:"decorator"`
	Decimals  int           `json:"decimals"`
	Activity  string        `json:"activity"`
	Source    string        `json:"source"`
	token     string        `json:"-"`
	close     chan int      `json:"-"`
}

// NewToken saves information about the stock and starts up a watcher on it
func NewToken(network string, contract string, token string, name string, nickname bool, frequency int, decimals int, activity string, color bool, decorator string, source string) *Token {
	m := &Token{
		Network:   network,
		Contract:  contract,
		Name:      name,
		Nickname:  nickname,
		Frequency: time.Duration(frequency) * time.Second,
		Color:     color,
		Decorator: decorator,
		Activity:  activity,
		Source:    source,
		token:     token,
		close:     make(chan int, 1),
	}

	// spin off go routine to watch the price
	go m.watchTokenPrice()
	return m
}

// Shutdown sends a signal to shut off the goroutine
func (m *Token) Shutdown() {
	m.close <- 1
}

func (m *Token) watchTokenPrice() {

	// create a new discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + m.token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	// show as online
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening discord connection,", err)
		return
	}

	// get bot id
	botUser, err := dg.User("@me")
	if err != nil {
		logger.Errorf("Getting bot id: %s", err)
		return
	}

	// Get guides for bot
	guilds, err := dg.UserGuilds(100, "", "")
	if err != nil {
		fmt.Println("Error getting guilds: ", err)
		m.Nickname = false
	}

	// Set arrows if no custom decorator
	var arrows bool
	if m.Decorator == "" {
		arrows = true
	}

	// Grab custom activity messages
	var custom_activity []string
	itr := 0
	itrSeed := 0.0
	if m.Activity != "" {
		custom_activity = strings.Split(m.Activity, ";")
	}

	logger.Infof("Watching token price for %s", m.Name)
	ticker := time.NewTicker(m.Frequency)

	// continuously watch
	var oldPrice float64
	for {
		select {
		case <-m.close:
			logger.Infof("Shutting down price watching for %s", m.Name)
			return
		case <-ticker.C:
			logger.Infof("Fetching stock price for %s", m.Name)
			var priceData string
			var fmtPriceRaw float64
			var fmtPrice float64

			switch m.Source {
			case "pancakeswap":
				logger.Infof("Using %s to get price: %s", m.Source, m.Name)

				// Get price from Ps in BNB
				priceData, err = utils.GetPancakeTokenPrice(m.Contract)
				if err != nil {
					logger.Errorf("Unable to fetch token price from %s: %s", m.Source, m.Name)
					continue
				}

				bnbRate, err := utils.GetCryptoPrice("binancecoin")
				if err != nil {
					logger.Errorf("Unable to fetch bnb price for %s", m.Name)
					continue
				}

				if fmtPriceRaw, err = strconv.ParseFloat(priceData, 64); err != nil {
					logger.Errorf("Error with price format for %s", m.Name)
				}
				fmtPrice = bnbRate.MarketData.CurrentPrice.USD * fmtPriceRaw

			default:
				priceData, err = utils.Get1inchTokenPrice(m.Network, m.Contract)
				if err != nil {
					logger.Errorf("Unable to fetch token price for %s", m.Name)
				}

				if fmtPriceRaw, err = strconv.ParseFloat(priceData, 64); err != nil {
					logger.Errorf("Error with price format for %s", m.Name)
				}
				fmtPrice = fmtPriceRaw / 10000000
			}

			// calculate if price has moved up or down
			var increase bool
			if fmtPrice >= oldPrice {
				increase = true
			} else {
				increase = false
			}

			if arrows {
				m.Decorator = "⬊"
				if increase {
					m.Decorator = "⬈"
				}
			}

			if m.Nickname {
				// update nickname instead of activity
				var nickname string
				var activity string

				// format nickname & activity
				// Check for custom decimal places
				switch m.Decimals {
				case 1:
					nickname = fmt.Sprintf("%s %s $%.1f", m.Name, m.Decorator, fmtPrice)
				case 2:
					nickname = fmt.Sprintf("%s %s $%.2f", m.Name, m.Decorator, fmtPrice)
				case 3:
					nickname = fmt.Sprintf("%s %s $%.3f", m.Name, m.Decorator, fmtPrice)
				case 4:
					nickname = fmt.Sprintf("%s %s $%.4f", m.Name, m.Decorator, fmtPrice)
				case 5:
					nickname = fmt.Sprintf("%s %s $%.5f", m.Name, m.Decorator, fmtPrice)
				case 6:
					nickname = fmt.Sprintf("%s %s $%.6f", m.Name, m.Decorator, fmtPrice)
				case 7:
					nickname = fmt.Sprintf("%s %s $%.7f", m.Name, m.Decorator, fmtPrice)
				case 8:
					nickname = fmt.Sprintf("%s %s $%.8f", m.Name, m.Decorator, fmtPrice)
				case 9:
					nickname = fmt.Sprintf("%s %s $%.9f", m.Name, m.Decorator, fmtPrice)
				case 10:
					nickname = fmt.Sprintf("%s %s $%.10f", m.Name, m.Decorator, fmtPrice)
				case 11:
					nickname = fmt.Sprintf("%s %s $%.11f", m.Name, m.Decorator, fmtPrice)
				default:
					nickname = fmt.Sprintf("%s %s $%.4f", m.Name, m.Decorator, fmtPrice)
				}

				activity = "Using USDC on 1inch"

				// Update nickname in guilds
				for _, g := range guilds {
					err = dg.GuildMemberNickname(g.ID, "@me", nickname)
					if err != nil {
						fmt.Println("Error updating nickname: ", err)
						continue
					}
					logger.Infof("Set nickname in %s: %s", g.Name, nickname)

					if m.Color {
						// get roles for colors
						var redRole string
						var greeenRole string

						roles, err := dg.GuildRoles(g.ID)
						if err != nil {
							logger.Errorf("Getting guilds: %s", err)
							continue
						}

						// find role ids
						for _, r := range roles {
							if r.Name == "tickers-red" {
								redRole = r.ID
							} else if r.Name == "tickers-green" {
								greeenRole = r.ID
							}
						}

						if len(redRole) == 0 || len(greeenRole) == 0 {
							logger.Error("Unable to find roles for color changes")
							continue
						}

						// assign role based on change
						if increase {
							err = dg.GuildMemberRoleRemove(g.ID, botUser.ID, redRole)
							if err != nil {
								logger.Errorf("Unable to remove role: %s", err)
							}
							err = dg.GuildMemberRoleAdd(g.ID, botUser.ID, greeenRole)
							if err != nil {
								logger.Errorf("Unable to set role: %s", err)
							}
						} else {
							err = dg.GuildMemberRoleRemove(g.ID, botUser.ID, greeenRole)
							if err != nil {
								logger.Errorf("Unable to remove role: %s", err)
							}
							err = dg.GuildMemberRoleAdd(g.ID, botUser.ID, redRole)
							if err != nil {
								logger.Errorf("Unable to set role: %s", err)
							}
						}
					}
				}

				// Custom activity messages
				if len(custom_activity) > 0 {

					// Display the real activity once per cycle
					if itr == len(custom_activity) {
						itr = 0
						itrSeed = 0.0
					} else if math.Mod(itrSeed, 2.0) == 1.0 {
						activity = custom_activity[itr]
						itr++
						itrSeed++
					} else {
						activity = custom_activity[itr]
						itrSeed++
					}
				}

				err = dg.UpdateGameStatus(0, activity)
				if err != nil {
					logger.Error("Unable to set activity: ", err)
				} else {
					logger.Infof("Set activity: %s", activity)
				}

			} else {
				activity := fmt.Sprintf("%s %s $%.2f", m.Name, m.Decorator, fmtPrice)

				err = dg.UpdateGameStatus(0, activity)
				if err != nil {
					logger.Error("Unable to set activity: ", err)
				} else {
					logger.Infof("Set activity: %s", activity)
				}
			}
			oldPrice = fmtPrice
		}
	}
}
