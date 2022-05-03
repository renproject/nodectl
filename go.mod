module github.com/renproject/nodectl

go 1.16

require (
	github.com/aws/aws-sdk-go v1.44.5
	github.com/digitalocean/godo v1.79.0
	github.com/ethereum/go-ethereum v1.10.16
	github.com/fatih/color v1.12.0
	github.com/google/go-github/v44 v44.0.0
	github.com/hashicorp/go-version v1.4.0
	github.com/hashicorp/hcl/v2 v2.12.0
	github.com/joho/godotenv v1.4.0
	github.com/renproject/aw v0.6.1
	github.com/renproject/id v0.4.2
	github.com/renproject/multichain v0.5.8
	github.com/renproject/pack v0.2.12
	github.com/renproject/surge v1.2.7
	github.com/urfave/cli/v2 v2.3.0
	github.com/zclconf/go-cty v1.8.4
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/oauth2 v0.0.0-20220411215720-9780585627b5
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
