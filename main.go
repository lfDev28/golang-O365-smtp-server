package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"os"
	"strings"
	"text/template"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type Request struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Message  string `json:"message"`
	URL      string `json:"url"`
}

type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(a.username), nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("unknown from server")
		}
	}
	return nil, nil
}

func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	bodyReader := strings.NewReader(request.Body)
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       "Failed at reading body",
			StatusCode: 500,
		}, nil
	}

	var req Request
	err = json.Unmarshal(body, &req)
	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       err.Error(),
			StatusCode: 500,
		}, nil
	}
	if req.Sender == "" || req.Receiver == "" || req.Message == "" || req.URL == "" {
		return events.APIGatewayProxyResponse{
			Body:       "Missing fields",
			StatusCode: 400,
		}, nil
	}

	from := os.Getenv("EMAIL")
	password := os.Getenv("PASSWORD")

	to := []string{
		req.Receiver,
	}

	smtpHost := "smtp.office365.com"
	smtpPort := "587"

	conn, err := net.Dial("tcp", "smtp.office365.com:587")
	if err != nil {
		println(err)
	}

	c, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		println(err)
	}

	tlsconfig := &tls.Config{
		ServerName: smtpHost,
	}

	if err = c.StartTLS(tlsconfig); err != nil {
		println(err)
	}

	auth := LoginAuth(from, password)

	if err = c.Auth(auth); err != nil {
		println(err)
	}

	t, _ := template.ParseFiles("template.html")

	var emailBody bytes.Buffer

	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	emailBody.Write([]byte(fmt.Sprintf("Subject: Tecsim Notification \n%s\n\n", mimeHeaders)))

	t.Execute(&emailBody, struct {
		Sender  string
		Message string
		URL     string
	}{
		Sender:  req.Sender,
		Message: req.Message,
		URL:     req.URL,
	})

	// Sending email.
	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, from, to, emailBody.Bytes())
	if err != nil {
		fmt.Println(err)
		return events.APIGatewayProxyResponse{
			Body:       "Failed at sending email",
			StatusCode: 500,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		Body:       "Email sent",
		StatusCode: 200,
	}, nil

}

func main() {
	lambda.Start(Handler)
}
