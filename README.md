gosmtpd
=======

Golang SMTP daemon library (smtpd)
gosmtpd receives email and send the messages to a channel. 

```go
import "github.com/slyngshede/gosmtpd"
import "net/mail"

config := smtpd.Config{Bind: ":2525",
                       Host: "smtp.localhost.localdomain",
                       Maxsize: 131072}

go smtpd.Serve(config, c)
```
