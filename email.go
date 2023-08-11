package email

import (
    "os"
    "path"
    "strings"
    "bytes"
    "fmt"
    "encoding/json"
    "encoding/base64"
    "net/http"
    "net/smtp"
    "crypto/tls"
)

type EmailCredentials struct {
    ClientId     string `json:"client_id"`
    ClientSecret string `json:"client_secret"`
    RefreshToken string `json:"refresh_token"`
}

type attachment struct {
    filename string
    data string // base64 encoded file data
}

// globals
var emailCredentials *EmailCredentials
var fromAddr string
var initialized = false

func Init(fromAddress, credentialsFilepath string) error {
	if initialized {
		return nil 
	}

    emailCredentials = new(EmailCredentials)
    fromAddr = fromAddress

    fileData, err := os.ReadFile(credentialsFilepath)
    if err != nil {
        return err 
    }

    if err = json.Unmarshal(fileData, emailCredentials); err != nil {
        return err
    }

	initialized = true
    return nil
}

func Send(subject string, messages []string, attachments []string, recipients []string) error {
    if emailCredentials == nil {
        return fmt.Errorf("Error: emailCredentials is nil. Make sure you call Init before calling Send")
    }

    accessToken, err := fetchGmailAccessToken()
    if err != nil {
        return fmt.Errorf("Error: failed to fetch gmail access token: %s\n", err)
    }


    authString := generateOauthString(fromAddr, accessToken)

    msg, err := emailMessage(subject, messages, attachments, recipients)
    if err != nil {
        return err
    }

    smtpServer := "smtp.gmail.com:587"
    client, err := smtp.Dial(smtpServer)
    if err != nil {
        return fmt.Errorf("Error: failed to connect to %s: %s\n", smtpServer, err)
    }
    defer client.Close()

    if err = client.Hello(emailCredentials.ClientId); err != nil {
        return err
    }

    if ok, _ := client.Extension("STARTTLS"); ok {
        config := &tls.Config{ServerName: "smtp.gmail.com"}
        if err = client.StartTLS(config); err != nil {
            return err
        }
    }

    if ok, _ := client.Extension("AUTH"); !ok {
        return fmt.Errorf("Error: smtp server doesn't support AUTH")
    }

    _, _, err = cmd(client, 235, "AUTH XOAUTH2 %s", authString)
    if err != nil {
        return fmt.Errorf("Error: failed to authenticate to smtp server with oauth2: %s", err)
    }


    //client.Hello(emailCredentials.ClientId)

    if err = client.Mail(fromAddr); err != nil {
        return err
    }

    for _, addr := range recipients {
        if err = client.Rcpt(addr); err != nil {
            return err
        }
    }

    // Write the email message
    w, err := client.Data()
    if err != nil {
        return err
    }
    _, err = w.Write(msg)
    if err != nil {
        return err
    }
    err = w.Close()
    if err != nil {
        return err
    }

    return client.Quit()
}





// NOTE(shaw): This function is yanked from the golang smtp source code
// I need to issue arbitrary smtp commands, but the smtp package doesn't export
// this functionality, so I am reimplementing it here
// https://cs.opensource.google/go/go/+/refs/tags/go1.19:src/net/smtp/smtp.go
func cmd(c *smtp.Client, expectCode int, format string, args ...interface{}) (int, string, error) {
	id, err := c.Text.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}
	c.Text.StartResponse(id)
	defer c.Text.EndResponse(id)
	code, msg, err := c.Text.ReadResponse(expectCode)
	return code, msg, err
}

// see https://en.wikipedia.org/wiki/MIME
func mimeMessage(subject string, nonMimeMessage string, message string, attachments []attachment, recipients []string) []byte {
    boundary := "|_oodilally_|"

    // get the first recipient and CC recipients
    var firstRecipient string
    var cc string
    if len(recipients) > 0 {
        firstRecipient = recipients[0]
        ccAddrs := recipients[1:]
        for i, addr := range ccAddrs {
            cc += addr
            if i < len(ccAddrs) - 1 {
                cc += ", "
            }
        }
    }

    var b strings.Builder

    // header
    b.WriteString(fmt.Sprintf("From: \"Otto\" <%s>\r\n", fromAddr))
    b.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
    if firstRecipient != "" {
        b.WriteString(fmt.Sprintf("To: <%s>\r\n", firstRecipient))
    }
    if cc != "" {
        b.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
    }
    b.WriteString("MIME-Version: 1.0\r\n")
    b.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary))
    b.WriteString(fmt.Sprintf("%s\r\n", nonMimeMessage))

    // email body
    b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
    b.WriteString("Content-Type: text/plain\r\n\r\n")
    b.WriteString(fmt.Sprintf("%s\r\n", message))

    // attachments
    for _, attachment := range attachments {
    b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
        b.WriteString("Content-Type: application/octet-stream; ")
        b.WriteString(fmt.Sprintf("name=\"%s\"\r\n", attachment.filename))
        b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
        b.WriteString(fmt.Sprintf("%s\r\n\r\n", attachment.data))
    }

    // close
    b.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
    b.WriteString(".\r\n")

    return []byte(b.String())
}

func emailMessage(subject string, messages, attachmentPaths, recipients []string) (message []byte, err error) {
    var messageString string

    for _, m := range messages {
        messageString += m + "\r\n"
    }

    var attachments []attachment
    for _, filepath := range attachmentPaths {
        attachmentData, err := os.ReadFile(filepath)
        if err != nil {
            return message, err
        }

        attachments = append(attachments, attachment{ 
            path.Base(filepath),
            base64.StdEncoding.EncodeToString(attachmentData) })
    }

    return mimeMessage(subject, messageString, messageString, attachments, recipients), nil
}

func fetchGmailAccessToken() (accessToken string, err error) {
    body, err := json.Marshal(map[string]string{
        "client_id": emailCredentials.ClientId,
        "client_secret": emailCredentials.ClientSecret,
        "refresh_token": emailCredentials.RefreshToken,
        "grant_type": "refresh_token", 
    })
    if err != nil {
        return accessToken, err
    }

	resp, err := http.Post("https://accounts.google.com/o/oauth2/token", "application/json", bytes.NewBuffer(body))
	if err != nil {
        return accessToken, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
        return accessToken, fmt.Errorf("Error: failed to fetch gmail access token. Reponse status is %d", resp.StatusCode)
	}

    var result struct {
        AccessToken string `json:"access_token"`
    }

	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return accessToken, err
    }

    return result.AccessToken, nil
}

func generateOauthString(username string, accessToken string) string {
    authString := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", username, accessToken)
    return base64.StdEncoding.EncodeToString([]byte(authString))
}



