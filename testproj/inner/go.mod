module github.com/example/complexmod

go 1.13

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/sirupsen/logrus v1.9.0
    golang.org/x/crypto v0.21.0
    golang.org/x/net v0.22.0
    golang.org/x/sys v0.18.0
    google.golang.org/protobuf v1.33.0
)

require (
    github.com/stretchr/testify v1.8.4 // indirect
    github.com/pmezard/go-difflib v1.0.0 // indirect
)

replace golang.org/x/sys => golang.org/x/sys v0.17.0

