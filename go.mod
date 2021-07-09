module github.com/renproject/nodectl

go 1.16

require (
	github.com/aws/aws-sdk-go v1.38.71
	github.com/digitalocean/godo v1.62.0
	github.com/ethereum/go-ethereum v1.10.3
	github.com/fatih/color v1.12.0
	github.com/google/go-github/v36 v36.0.0
	github.com/hashicorp/go-version v1.3.0
	github.com/hashicorp/hcl/v2 v2.10.0
	github.com/renproject/aw v0.4.1-0.20210604011747-50d6a643dc76
	github.com/renproject/id v0.4.2
	github.com/renproject/multichain v0.3.16
	github.com/renproject/pack v0.2.11
	github.com/renproject/surge v1.2.6
	github.com/urfave/cli/v2 v2.3.0
	github.com/zclconf/go-cty v1.8.4
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
)

replace github.com/cosmos/ledger-cosmos-go => github.com/terra-project/ledger-terra-go v0.11.1-terra

replace github.com/CosmWasm/go-cosmwasm => github.com/terra-project/go-cosmwasm v0.10.1-terra

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4