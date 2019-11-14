module github.com/austinabell/etherminty

go 1.13

require (
	github.com/aristanetworks/goarista v0.0.0-20191023202215-f096da5361bb // indirect
	github.com/cosmos/cosmos-sdk v0.34.4-0.20191031200835-02c6c9fafd58
	github.com/cosmos/ethermint v0.0.0-20190802135314-3f32f9ba8a1f
	github.com/elastic/gosigar v0.10.5 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.5.0
	github.com/stretchr/testify v1.4.0
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/tendermint v0.32.7
	github.com/tendermint/tm-db v0.2.0
)

replace github.com/cosmos/ethermint v0.0.0-20190802135314-3f32f9ba8a1f => github.com/chainsafe/ethermint v0.0.0-20191114211431-4401a8c6beeb
