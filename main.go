package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
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
	checkErr(err)

	db, err := sql.Open("mysql", os.Getenv("DATABASE_URL"))
	checkErr(err)

	err = db.Ping()
	checkErr(err)

	defer db.Close()

	go startAPIServer(db, os.Getenv("LISTEN"))

	bot := tbot.New(os.Getenv("TELEGRAM_TOKEN"))
	c := bot.Client()

	bot.HandleMessage("/", func(m *tbot.Message) {
		c.SendChatAction(m.Chat.ID, tbot.ActionTyping)
		time.Sleep(1 * time.Second)
		markup := tbot.Buttons([][]string{
			{"last", "day"},
		})
		c.SendMessage(m.Chat.ID, "Pick an option:", tbot.OptReplyKeyboardMarkup(markup))
	})

	bot.HandleMessage("last", func(m *tbot.Message) {
		c.SendChatAction(m.Chat.ID, tbot.ActionTyping)
		time.Sleep(1 * time.Second)
		measurement := new(Measurement)
		row := db.QueryRow("SELECT created_at, temperature, humidity FROM measurements ORDER BY created_at DESC LIMIT 1")
		err = row.Scan(&measurement.CreatedAt, &measurement.Temperature, &measurement.Humidity)
		if err == sql.ErrNoRows {
			c.SendMessage(m.Chat.ID, "No mesaurements :(")
		}
		checkErr(err)
		msg := fmt.Sprintf(
			"[%s]   %.1f°C   %.1f%%",
			measurement.CreatedAt.Format("15:04 Jan _2"),
			measurement.Temperature,
			measurement.Humidity,
		)
		c.SendMessage(m.Chat.ID, msg)
	})

	bot.HandleMessage("day", func(m *tbot.Message) {
		c.SendChatAction(m.Chat.ID, tbot.ActionTyping)
		time.Sleep(1 * time.Second)
		rows, err := db.Query("SELECT created_at, temperature, humidity FROM measurements WHERE created_at >= now() - INTERVAL 1 DAY")
		checkErr(err)
		defer rows.Close()
		var temperatures []float64
		var humidities []float64
		var dates []time.Time
		for rows.Next() {
			measurement := new(Measurement)
			err := rows.Scan(&measurement.CreatedAt, &measurement.Temperature, &measurement.Humidity)
			checkErr(err)
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
		checkErr(err)
	})

	err = bot.Start()
	checkErr(err)
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

func startAPIServer(db *sql.DB, listen string) {
	http.HandleFunc("/dht22", func(w http.ResponseWriter, r *http.Request) {
		t, err := strconv.ParseFloat(r.FormValue("t"), 64)
		if err != nil {
			httpErr("Can not parse temperature", 400, w)
			return
		}
		h, err := strconv.ParseFloat(r.FormValue("h"), 64)
		if err != nil {
			httpErr("Can not parse humidity", 400, w)
			return
		}
		sql := "INSERT INTO measurements(created_at, temperature, humidity) values (?, ?, ?)"
		stmt, err := db.Prepare(sql)
		if err != nil {
			httpErr("Can not create sql statement", 500, w)
			return
		}
		_, err = stmt.Exec(time.Now(), t, h)
		if err != nil {
			httpErr("Can not insert values into database", 500, w)
			return
		}
		fmt.Fprint(w, "OK")
	})
	fmt.Printf("Starting server at: %s\n", listen)
	if err := http.ListenAndServe(listen, nil); err != nil {
		log.Fatal(err)
	}
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func httpErr(msg string, code int, w http.ResponseWriter) {
	http.Error(w, msg, code)
}
