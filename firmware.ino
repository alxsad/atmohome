#include <ESP8266WiFi.h>
#include <WiFiClient.h>
#include <ESP8266HTTPClient.h>
#include <Adafruit_BME280.h>
#include <Adafruit_Sensor.h>

#define SERIAL_SPEED   9600
#define WIFI_SSID      "skynet2"
#define WIFI_PASS      "password"

// давления на уровне моря
#define SEALEVELPRESSURE_HPA (1013.25)

Adafruit_BME280 bme;
float t, h, p, a;
int v;
String server = "http://192.168.1.105:8080/dht22";
HTTPClient http;

ADC_MODE(ADC_VCC);

void setup(void) {
  Serial.begin(SERIAL_SPEED);
  while (!Serial) {}

  bme.begin(0x76);

  WiFi.persistent(false);
  WiFi.mode(WIFI_OFF);
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASS);

  while (WiFi.status() != WL_CONNECTED) {
    delay(100);
  }
  Serial.println(WiFi.localIP());

  t = bme.readTemperature();
  h = bme.readHumidity();
  p = bme.readPressure() / 100.0F;
  a = bme.readAltitude(SEALEVELPRESSURE_HPA);
  v = ESP.getVcc();

  Serial.print("Temperature: ");
  Serial.println(t);
  Serial.print("Humidity: ");
  Serial.println(h);
  Serial.print("Pressure: ");
  Serial.println(p);
  Serial.print("Altitude: ");
  Serial.println(a);
  Serial.print("VCC: ");
  Serial.println(v);

  String url = server + "?t=" + t + "&h=" + h + "&p=" + p + "&a=" + a + "&v=" + v;
  http.begin(url.c_str());
  int httpCode = http.GET();
  http.end();
  Serial.println(httpCode);
  
  ESP.deepSleep(300e6);
}

void loop(void) { 
}
