import Adafruit_DHT
import mysql.connector
from datetime import date, datetime

DHT_SENSOR = Adafruit_DHT.DHT22
DHT_PIN = 25

db = mysql.connector.connect(
    host="localhost",
    user="dht22",
    passwd="dht22",
    database="dht22"
)
cursor = db.cursor()
created = datetime.now()

h, t = Adafruit_DHT.read_retry(DHT_SENSOR, DHT_PIN)
sql = "INSERT INTO measurements (created_at, temperature, humidity) VALUES (%s, %s, %s)"
val = (created, round(t, 2), round(h, 2))
cursor.execute(sql, val)
db.commit()
print("t={0:0.1f} h={1:0.1f}".format(t, h))
