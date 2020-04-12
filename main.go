package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/wcharczuk/go-chart"
	"github.com/yanzay/tbot/v2"
)

// Measurement ...
type Measurement struct {
	CreatedAt   time.Time `json:"created_at"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
}

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Panic(err)
	}

	db, err := sql.Open("mysql", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	bot := tbot.New(os.Getenv("TELEGRAM_TOKEN"))
	c := bot.Client()

	bot.HandleMessage("/last", func(m *tbot.Message) {
		c.SendChatAction(m.Chat.ID, tbot.ActionTyping)
		time.Sleep(1 * time.Second)
		measurement := new(Measurement)
		row := db.QueryRow("SELECT created_at, temperature, humidity FROM measurements ORDER BY created_at DESC LIMIT 1")
		err = row.Scan(&measurement.CreatedAt, &measurement.Temperature, &measurement.Humidity)
		if err == sql.ErrNoRows {
			c.SendMessage(m.Chat.ID, "No mesaurements :(")
		}
		if err != nil {
			log.Fatal(err)
		}
		msg := fmt.Sprintf(
			"Temperature = %.1f°C | Humidity = %.1f%% | Time = %s",
			measurement.Temperature,
			measurement.Humidity,
			measurement.CreatedAt.Format("15:04 Jan _2"),
		)
		c.SendMessage(m.Chat.ID, msg)
	})

	bot.HandleMessage("/day", func(m *tbot.Message) {
		c.SendChatAction(m.Chat.ID, tbot.ActionTyping)
		time.Sleep(1 * time.Second)

		rows, err := db.Query("SELECT created_at, temperature, humidity FROM measurements WHERE created_at >= now() - INTERVAL 1 DAY")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		var temperatures []float64
		var humidities []float64
		var dates []time.Time
		for rows.Next() {
			measurement := new(Measurement)
			err := rows.Scan(&measurement.CreatedAt, &measurement.Temperature, &measurement.Humidity)
			if err != nil {
				log.Fatal(err)
			}
			dates = append(dates, measurement.CreatedAt)
			temperatures = append(temperatures, measurement.Temperature)
			humidities = append(humidities, measurement.Humidity)
		}

		graph := chart.Chart{
			XAxis: chart.XAxis{
				ValueFormatter: func(v interface{}) string {
					return formatTime(v, "15:04")
				},
			},
			Series: []chart.Series{
				chart.TimeSeries{
					Name:    "Temperature",
					XValues: dates,
					YValues: temperatures,
				},
				chart.TimeSeries{
					Name:    "Humidity",
					XValues: dates,
					YValues: humidities,
				},
			},
		}
		graph.Elements = []chart.Renderable{
			chart.LegendThin(&graph),
		}

		f, _ := os.Create("output.png")
		defer f.Close()
		err = graph.Render(chart.PNG, f)

		_, err = c.SendPhotoFile(m.Chat.ID, "output.png", tbot.OptCaption("Last 24 hours graph"))
		if err != nil {
			log.Fatal(err)
		}
	})

	err = bot.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func formatTime(v interface{}, dateFormat string) string {
	if typed, isTyped := v.(time.Time); isTyped {
		return typed.Format(dateFormat)
	}
	if typed, isTyped := v.(int64); isTyped {
		return time.Unix(0, typed).Format(dateFormat)
	}
	if typed, isTyped := v.(float64); isTyped {
		return time.Unix(0, int64(typed)).Format(dateFormat)
	}
	return ""
}
