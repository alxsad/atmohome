package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wcharczuk/go-chart"
	"github.com/yanzay/tbot/v2"
)

// Measurement ...
type Measurement struct {
	CreatedAt   time.Time `json:"created_at"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Pressure    float64   `json:"pressure"`
	Altitude    float64   `json:"altittude"`
	Vcc         float64   `json:"vcc"`
}

var msgSent = false

func main() {

	err := godotenv.Load()
	checkErr(err)

	db, err := sql.Open("sqlite3", os.Getenv("DATABASE_URL"))
	checkErr(err)

	err = db.Ping()
	checkErr(err)

	defer db.Close()

	go startAPIServer(db, os.Getenv("LISTEN"))

	bot := tbot.New(os.Getenv("TELEGRAM_TOKEN"))
	c := bot.Client()

	bot.HandleMessage("/", func(msg *tbot.Message) {
		c.SendChatAction(msg.Chat.ID, tbot.ActionTyping)
		markup := tbot.Buttons([][]string{
			{"last", "day", "pressure", "vcc"},
		})
		c.SendMessage(msg.Chat.ID, "Pick an option:", tbot.OptReplyKeyboardMarkup(markup))
	})

	bot.HandleMessage("last", func(msg *tbot.Message) {
		c.SendChatAction(msg.Chat.ID, tbot.ActionTyping)
		m := new(Measurement)
		row := db.QueryRow("SELECT created_at, temperature, humidity, pressure, altitude FROM measurements ORDER BY created_at DESC LIMIT 1")
		err = row.Scan(&m.CreatedAt, &m.Temperature, &m.Humidity, &m.Pressure, &m.Altitude)
		if err == sql.ErrNoRows {
			c.SendMessage(msg.Chat.ID, "No mesaurements :(")
		}
		checkErr(err)
		response := fmt.Sprintf(
			"[%s]  %.1f°C  %.1f%%  %.1f Pa",
			m.CreatedAt.Format("15:04 Jan _2"),
			m.Temperature,
			m.Humidity,
			m.Pressure,
		)
		c.SendMessage(msg.Chat.ID, response)
	})

	bot.HandleMessage("vcc", func(msg *tbot.Message) {
		c.SendChatAction(msg.Chat.ID, tbot.ActionTyping)
		m := new(Measurement)
		row := db.QueryRow("SELECT created_at, vcc FROM measurements ORDER BY created_at DESC LIMIT 1")
		err = row.Scan(&m.CreatedAt, &m.Vcc)
		if err == sql.ErrNoRows {
			c.SendMessage(msg.Chat.ID, "No VCC :(")
		}
		checkErr(err)
		response := fmt.Sprintf(
			"[%s] %.2f",
			m.CreatedAt.Format("15:04 Jan _2"),
			m.Vcc,
		)
		c.SendMessage(msg.Chat.ID, response)
	})

	bot.HandleMessage("day", func(msg *tbot.Message) {
		c.SendChatAction(msg.Chat.ID, tbot.ActionTyping)
		rows, err := db.Query("SELECT created_at, temperature, humidity FROM measurements WHERE created_at >= datetime('now', '-1 day')")
		checkErr(err)
		defer rows.Close()
		var temperatures []float64
		var humidities []float64
		var dates []time.Time
		for rows.Next() {
			m := new(Measurement)
			err := rows.Scan(&m.CreatedAt, &m.Temperature, &m.Humidity)
			checkErr(err)
			dates = append(dates, m.CreatedAt)
			temperatures = append(temperatures, m.Temperature)
			humidities = append(humidities, m.Humidity)
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

		_, err = c.SendPhotoFile(msg.Chat.ID, "output.png", tbot.OptCaption("Last 24 hours graph"))
		checkErr(err)
	})

	bot.HandleMessage("pressure", func(msg *tbot.Message) {
		c.SendChatAction(msg.Chat.ID, tbot.ActionTyping)
		rows, err := db.Query("SELECT created_at, pressure FROM measurements WHERE created_at >= datetime('now', '-1 day')")
		checkErr(err)
		defer rows.Close()
		var pressures []float64
		var dates []time.Time
		for rows.Next() {
			m := new(Measurement)
			err := rows.Scan(&m.CreatedAt, &m.Pressure)
			checkErr(err)
			dates = append(dates, m.CreatedAt)
			pressures = append(pressures, m.Pressure)
		}

		graph := chart.Chart{
			XAxis: chart.XAxis{
				ValueFormatter: func(v interface{}) string {
					return formatTime(v, "15:04")
				},
			},
			Series: []chart.Series{
				chart.TimeSeries{
					Name:    "Pressure",
					XValues: dates,
					YValues: pressures,
				},
			},
		}
		graph.Elements = []chart.Renderable{
			chart.LegendThin(&graph),
		}

		f, _ := os.Create("output.png")
		defer f.Close()
		err = graph.Render(chart.PNG, f)

		_, err = c.SendPhotoFile(msg.Chat.ID, "output.png", tbot.OptCaption("Last 24 hours graph"))
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
		fmt.Println("New measurement:", r.URL.Query())
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
		p, err := strconv.ParseFloat(r.FormValue("p"), 64)
		if err != nil {
			httpErr("Can not parse pressure", 400, w)
			return
		}
		a, err := strconv.ParseFloat(r.FormValue("a"), 64)
		if err != nil {
			httpErr("Can not parse altitude", 400, w)
			return
		}
		v, err := strconv.ParseFloat(r.FormValue("v"), 64)
		if err != nil {
			httpErr("Can not parse VCC", 400, w)
			return
		}
		sql := "INSERT INTO measurements(created_at, temperature, humidity, pressure, altitude, vcc) values (?, ?, ?, ?, ?, ?)"
		stmt, err := db.Prepare(sql)
		if err != nil {
			httpErr("Can not create sql statement", 500, w)
			return
		}
		_, err = stmt.Exec(time.Now(), t, h, p, a, v/1000)
		if err != nil {
			httpErr("Can not insert values into database", 500, w)
			return
		}
		if v < 2900 && !msgSent {
			msgSent = true
			go func() {
				_, _ = http.Post(
					"http://ntfy.sh/alxsad",
					"application/x-www-form-urlencoded",
					bytes.NewBuffer([]byte("🪫atmohome: charge me!🪫")),
				)
				time.Sleep(24 * time.Hour)
				msgSent = false
			}()
		}
		fmt.Fprint(w, "OK")
	})
	http.HandleFunc("/rows", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT created_at, temperature, humidity, pressure, altitude, vcc FROM measurements WHERE created_at >= datetime('now', '-1 day')")
		checkErr(err)
		defer rows.Close()
		result := []Measurement{}
		for rows.Next() {
			m := Measurement{}
			err := rows.Scan(&m.CreatedAt, &m.Temperature, &m.Humidity, &m.Pressure, &m.Altitude, &m.Vcc)
			checkErr(err)
			result = append(result, m)
		}
		b, err := json.Marshal(result)
		checkErr(err)
		fmt.Fprint(w, string(b))
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
