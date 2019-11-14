# Etherminty

Etherminty is the base application needed to use the EVM module within [Ethermint](https://github.com/ChainSafe/ethermint).

This can be used either as boilerplate to creating a Cosmos-sdk application/module that interacts with the EVM module, or just to identify the necessary changes to a base framework to allow compatibility with an existing application/module.

The changes necessary to make to use the EVM module are indicated with `// *` comments with explanations to why they are used.

Commands to start the node can vary, but the commands I ran to start the node are:
```
make install 
rm -rf ~/.emty*
emtyd init moniker --chain-id 8
emtycli config chain-id 8
emtycli config output json
emtycli config indent true
emtycli config trust-node true
echo "testpass" | emtycli keys add mykey
echo "testpass" | emtycli keys add mykey2
emtyd add-genesis-account $(emtycli keys show mykey -a) 1000000000000000000photon,1000000000000000000stake
emtyd add-genesis-account $(emtycli keys show mykey2 -a) 1000000000000000000photon,1000000000000000000stake
echo "testpass" | emtyd gentx --name mykey
emtyd collect-gentxs
emtyd validate-genesis
emtyd start --pruning=nothing

```