
package main

import (
    "bytes"
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
    "net/http"
    "net/url"
    "strings"
    "time"
    "crypto/tls"
)

var (
    rockerurl = "https://rocket.chat.ru.channel"
    Api_url   = "https://api_site"
)

var serverLocations = map[string]string{
    "1":  "RU",
    "12":  "NL",
    "123":  "FI",
    "1234":  "FR",
    "12345":  "UK",
    "123456":  "DE",
}

type Massage struct {
    Text string `json:"text"`
}

type CallBack struct {
    Result   string `json:"result"`
    Action   string `json:"action"`
    Callback string `json:"callback"`
}

type Scope2 struct {
    Result string `json:"result"`
    Scope  string `json:"scope"`
    Key    string `json:"key"`
}

func sendMessage(msg string) {
    m := Massage{Text: msg}
    data, _ := json.Marshal(m)
    client := &http.Client{Timeout: 15 * time.Second}
    req, _ := http.NewRequest("POST", rockerurl, bytes.NewBuffer(data))
    req.Header.Set("Content-Type", "application/json")
    client.Do(req)
}

func GetTokenUser() string {
    body := strings.NewReader("action=login&key=0364343a2e005291-3f8da7e11ed25f2f")
    req, _ := http.NewRequest("POST", Api_url+"auth.php", body)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    resp, _ := http.DefaultClient.Do(req)
    buf, _ := ioutil.ReadAll(resp.Body)
    defer resp.Body.Close()
    var msg struct {
        Result struct{ Token string } `json:"result"`
    }
    json.Unmarshal(buf, &msg)
    return msg.Result.Token
}

func Console(token, ID string) (string, error) {
    data := url.Values{}
    data.Set("action", "novnc")
    data.Set("token", token)
    data.Set("id", ID)
    data.Set("pin", "not_set")
    req, _ := http.NewRequest("POST", Api_url+"eq.php", strings.NewReader(data.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    buf, _ := ioutil.ReadAll(resp.Body)
    var cb CallBack
    err = json.Unmarshal(buf, &cb)
    return cb.Callback, err
}

func Status2(key string) (string, string, error) {
    data := url.Values{}
    data.Set("action", "check")
    data.Set("key", key)
    req, _ := http.NewRequest("POST", "callback.php", strings.NewReader(data.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", "", err
    }
    defer resp.Body.Close()
    buf, _ := ioutil.ReadAll(resp.Body)
    var s Scope2
    err = json.Unmarshal(buf, &s)
    return s.Result, s.Scope, err
}

func StopConsole(token, ID string) error {
    data := url.Values{}
    data.Set("action", "stop_novnc")
    data.Set("token", token)
    data.Set("id", ID)
    req, _ := http.NewRequest("POST", Api_url+"eq.php", strings.NewReader(data.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    _, err := http.DefaultClient.Do(req)
    return err
}

func POWER(token, ID string) (string, string, error) {
    tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
    client := &http.Client{Transport: tr}
    urlParams := url.Values{}
    urlParams.Add("action", "status")
    urlParams.Add("token", token)
    urlParams.Add("id", ID)
    body := strings.NewReader(urlParams.Encode())
    req, _ := http.NewRequest("POST", Api_url+"eq.php", body)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    resp, err := client.Do(req)
    if err != nil {
        return "", "", err
    }
    defer resp.Body.Close()
    buf, _ := ioutil.ReadAll(resp.Body)
    var msg CallBack
    json.Unmarshal(buf, &msg)
    return msg.Callback, msg.Result, nil
}


func main() {
    action := flag.String("action", "", "start_console | start_power")
    flag.Parse()

    token := GetTokenUser()
    servers := []string{"1", "12", "123", "1234", "12345", "123456"}

    if *action == "start_power" {
        var report []string
        for _, id := range servers {
            key, _, _ := POWER(token, id)
            for i := 0; i < 300; i++ {
                res, _, _ := Status2(key)
                if res == "OK" {
                    report = append(report, fmt.Sprintf("⚡ %s (%s): питание включено", id, serverLocations[id]))
                    break
                }
                time.Sleep(1 * time.Second)
                if i == 29 {
                    report = append(report, fmt.Sprintf("❌ %s (%s): питание не включилось (таймаут)", id, serverLocations[id]))
                }
            }
        }
        summary := "Отчёт по питанию серверов:\n" + strings.Join(report, "\n")
        sendMessage(summary)
        return
    }

    if *action == "start_console" {
        var report []string
        for _, id := range servers {
            var key string
            var err error

            timeout := time.After(5 * time.Minute)
            ticker := time.Tick(200 * time.Second)
            success := false

        startConsoleLoop:
            for {
                select {
                case <-timeout:
                    report = append(report, fmt.Sprintf("❌ %s (%s): не удалось запустить консоль (таймаут)", id, serverLocations[id]))
                    break startConsoleLoop
                case <-ticker:
                    key, err = Console(token, id)
                    if err == nil {
                        fmt.Printf("✅ Console API accepted for %s\n", id)
                        success = true
                        break startConsoleLoop
                    }
                    fmt.Printf("⏳ Console not ready for %s: %v\n", id, err)
                }
            }

            if !success {
                continue
            }

            var scope string
            ready := false
            for i := 0; i < 600; i++ {
                res, s, _ := Status2(key)
                if res == "OK" {
                    scope = s
                    ready = true
                    break
                }
                time.Sleep(1 * time.Second)
            }

            if !ready {
                report = append(report, fmt.Sprintf("⏳ %s (%s): не дождались готовности консоли", id, serverLocations[id]))
                continue
            }

            resp, err := http.Get(scope)
            if err != nil {
                report = append(report, fmt.Sprintf("❌ %s (%s): ошибка подключения к консоли", id, serverLocations[id]))
            } else {
                if resp.StatusCode == 200 || resp.StatusCode == 404 {
                    report = append(report, fmt.Sprintf("✅ %s (%s): консоль ОК", id, serverLocations[id]))
                } else {
                    report = append(report, fmt.Sprintf("❌ %s (%s): ошибка консоли (код %d)", id, serverLocations[id], resp.StatusCode))
                }
                resp.Body.Close()
            }

            err = StopConsole(token, id)
            if err != nil {
                report = append(report, fmt.Sprintf("⚠️ %s (%s): ошибка закрытия консоли", id, serverLocations[id]))
            }
        }

        summary := "Отчёт по проверке консолей:\n" + strings.Join(report, "\n")
        sendMessage(summary)
        return
    }

    if *action == "stop_console" {
        var report []string
        for _, id := range servers {
            err := StopConsole(token, id)
            if err != nil {
                report = append(report, fmt.Sprintf("❌ %s (%s): ошибка при закрытии консоли", id, serverLocations[id]))
            } else {
                report = append(report, fmt.Sprintf("🛑 %s (%s): консоль успешно закрыта", id, serverLocations[id]))
            }
        }
        summary := "Отчёт по закрытию консолей:\n" + strings.Join(report, "\n")
        sendMessage(summary)
        return
    }

    fmt.Println("Неизвестное действие. Используй -action start_console или start_power")
}

