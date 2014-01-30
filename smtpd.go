package smtpd

import (
    "bufio"
    "fmt"
    "io"
    "net"
    "net/mail"
    "strings"
    "time"
    )

type Client struct {
    conn        net.Conn
    bufin       *bufio.Reader
    bufout      *bufio.Writer
    state       int
    mail_from   string
    rcpt_to     string
    config      Config
}

type Config struct {
    Bind        string
    Host        string
    Maxsize     int
}

func (c Config) GetGreeting() string {
    return fmt.Sprintf("220 %s ESMTP GoSMTPd\r\n", c.Host)
}

func wresp(client *Client, msg string)(err error){
    _, err = client.bufout.WriteString(msg)
    if err != nil {
        return err
    }
    client.bufout.Flush()
    return nil
}

func processCmd(client *Client)(resp string, err error){
    resp = "500 unrecognized command"
    input, err := client.bufin.ReadString('\n')
    if err != nil{
        return resp, err
    }

    input = strings.Trim(input, " \r\n")
    cmd := strings.ToUpper(input)
    switch {
        case strings.Index(cmd, "HELO") == 0:
            resp = "250 " + client.config.Host + " Hello " + input[5:] + "\r\n"
        case strings.Index(cmd, "EHLO") == 0:
            remote := client.conn.RemoteAddr().String()
            resp = fmt.Sprintf("250 %s Hello %s [%s]\r\n250-SIZE %d\r\n",
            client.config.Host,
            input[5:], remote, client.config.Maxsize)
        case strings.Index(cmd, "QUIT") == 0:
            resp = "221 Bye\r\n"
            client.state = 3
        case strings.Index(cmd, "NOOP") == 0:
            resp = "250 OK\r\n"
        case strings.Index(cmd, "RSET") == 0:
            client.mail_from = ""
            client.rcpt_to = ""
            resp = "250 OK\r\n"
        case strings.Index(cmd, "MAIL FROM:") == 0:
            client.mail_from = input[10:]
            resp = "250 Ok\r\n"
        case strings.Index(cmd, "RCPT TO:") == 0:
            if len(input) > 8 {
                client.rcpt_to = input[8:]
                resp = "250 Ok\r\n"
            }
        case strings.Index(cmd, "DATA") == 0:
            resp = "354 Enter message, ending with \".\" on a line by itself\r\n"
            client.state = 2
        default:
            resp = "500 Too many unrecognized commands\r\n"

    }
    return resp, nil
}

func processData(client *Client)(data string, err error){
    data = ""
    err = nil
    terminate := "\r\n.\r\n"
    for err == nil {
        line, err := client.bufin.ReadString('\n')
        if err != nil{
           break
        }

        data = data + line
        if len(data) > client.config.Maxsize{
            return data, err
        }

        if strings.HasSuffix(data, terminate) {
            data = strings.Trim(data, terminate)
            break
        }
    }
    return data, err
}

func dataToMessage(data string)(msg *mail.Message, err error){
    r := strings.NewReader(data)
    msg, err = mail.ReadMessage(r)
    return msg, err
}

func handleConnection(conn net.Conn, config Config, ch chan *mail.Message){
    conn.SetDeadline(time.Now().Add(60 * time.Second))
    defer conn.Close()

    var client *Client
    client = &Client{conn: conn,
                      state: 0,
                      bufin: bufio.NewReader(conn),
                      bufout: bufio.NewWriter(conn),
                      config: config,
                    }

    var resp string

    for {
        var err error
        err = nil

        switch client.state {
            case 0:
                // Greeting
                err = wresp(client, client.config.GetGreeting())
                client.state = 1
            case 1:
                resp, err = processCmd(client)
                err = wresp(client, resp)
            case 2:
                // Data
                data, err := processData(client)
                if err != nil{
                    resp = "554 Error: "
                } else {
                    resp ="250 OK : queued as 1"
                    msg, err := dataToMessage(data)
                    if err == nil{
                        ch <- msg
                    }
                }
                err = wresp(client, resp)
                return
            case 3:
                // Quit
                return
        }

        if err == io.EOF {
            // client closed the connection already
            return
        }
        if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
            // Timeout
            return
        }
    }

}

func Serve(config Config, c chan *mail.Message){

    listener, err := net.Listen("tcp", config.Bind)
    if err != nil {
        //Connection error
    }

    for {
        conn, err := listener.Accept()
        if err != nil {
            //Handle listener error
            continue
        }

        go handleConnection(conn, config, c)
    }

}
