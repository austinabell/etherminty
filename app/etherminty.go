package app

import (
	"encoding/json"
	"os"

	"github.com/cosmos/ethermint/x/evm"

	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/supply"

	eminttypes "github.com/cosmos/ethermint/types"
	evmtypes "github.com/cosmos/ethermint/x/evm/types"

	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	tmlog "github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"
	tmtypes "github.com/tendermint/tendermint/types"
)

const appName = "Etherminty"

var (
	// DefaultCLIHome sets the default home directories for the application CLI
	DefaultCLIHome = os.ExpandEnv("$HOME/.emtycli")

	// DefaultNodeHome sets the folder where the applcation data and configuration will be stored
	DefaultNodeHome = os.ExpandEnv("$HOME/.emtyd")

	// ModuleBasics is the module BasicManager is in charge of setting up basic,
	// non-dependant module elements, such as codec registration
	// and genesis verification.
	ModuleBasics = module.NewBasicManager(
		genutil.AppModuleBasic{},
		auth.AppModuleBasic{},
		bank.AppModuleBasic{},
		staking.AppModuleBasic{},
		distr.AppModuleBasic{},
		params.AppModuleBasic{},
		slashing.AppModuleBasic{},
		supply.AppModuleBasic{},

		evm.AppModuleBasic{},
	)

	// module account permissions
	maccPerms = map[string][]string{
		auth.FeeCollectorName:     nil,
		distr.ModuleName:          nil,
		staking.BondedPoolName:    {supply.Burner, supply.Staking},
		staking.NotBondedPoolName: {supply.Burner, supply.Staking},
	}
)

// MakeCodec generates the necessary codecs for Amino
func MakeCodec() *codec.Codec {
	var cdc = codec.New()

	ModuleBasics.RegisterCodec(cdc)
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	// * Register evm account type in codec here
	eminttypes.RegisterCodec(cdc)

	return cdc
}

// EthermintyApp defines structure of data for the app
type EthermintyApp struct {
	*bam.BaseApp
	cdc *codec.Codec

	// keys to access the substores
	keys  map[string]*sdk.KVStoreKey
	tkeys map[string]*sdk.TransientStoreKey

	// keepers
	accountKeeper  auth.AccountKeeper
	bankKeeper     bank.Keeper
	stakingKeeper  staking.Keeper
	slashingKeeper slashing.Keeper
	distrKeeper    distr.Keeper
	supplyKeeper   supply.Keeper
	paramsKeeper   params.Keeper
	evmKeeper      evm.Keeper

	// module manager
	mm *module.Manager
}

// NewEthermintyApp returns a reference to a new initialized base Ethermint
// application
func NewEthermintyApp(
	logger tmlog.Logger, db dbm.DB, loadLatest bool, baseAppOptions ...func(*bam.BaseApp)) *EthermintyApp {
	// define codec to be used by modules
	cdc := MakeCodec()

	bApp := bam.NewBaseApp(appName, logger, db, evmtypes.TxDecoder(cdc), baseAppOptions...)
	bApp.SetAppVersion(version.Version)

	keys := sdk.NewKVStoreKeys(bam.MainStoreKey, auth.StoreKey, staking.StoreKey,
		supply.StoreKey, distr.StoreKey, slashing.StoreKey, params.StoreKey,
		// * Inlcude these two keys
		evmtypes.EvmStoreKey, evmtypes.EvmCodeKey)
	// * Include block key for now, only used by web3 api
	blockKey := sdk.NewKVStoreKey(evmtypes.EvmBlockKey)

	tkeys := sdk.NewTransientStoreKeys(staking.TStoreKey, params.TStoreKey)

	app := &EthermintyApp{
		BaseApp: bApp,
		cdc:     cdc,
		keys:    keys,
		tkeys:   tkeys,
	}

	// init params keeper and subspaces
	app.paramsKeeper = params.NewKeeper(app.cdc, keys[params.StoreKey], tkeys[params.TStoreKey], params.DefaultCodespace)
	authSubspace := app.paramsKeeper.Subspace(auth.DefaultParamspace)
	bankSubspace := app.paramsKeeper.Subspace(bank.DefaultParamspace)
	stakingSubspace := app.paramsKeeper.Subspace(staking.DefaultParamspace)
	distrSubspace := app.paramsKeeper.Subspace(distr.DefaultParamspace)
	slashingSubspace := app.paramsKeeper.Subspace(slashing.DefaultParamspace)

	// add keepers
	app.accountKeeper = auth.NewAccountKeeper(app.cdc, keys[auth.StoreKey], authSubspace,
		// * override prototype base account to use account in evm
		eminttypes.ProtoBaseAccount)

	app.bankKeeper = bank.NewBaseKeeper(app.accountKeeper, bankSubspace,
		bank.DefaultCodespace, app.ModuleAccountAddrs())

	app.supplyKeeper = supply.NewKeeper(app.cdc, keys[supply.StoreKey], app.accountKeeper,
		app.bankKeeper, maccPerms)

	stakingKeeper := staking.NewKeeper(app.cdc, keys[staking.StoreKey],
		app.supplyKeeper, stakingSubspace, staking.DefaultCodespace)

	app.distrKeeper = distr.NewKeeper(app.cdc, keys[distr.StoreKey], distrSubspace, &stakingKeeper,
		app.supplyKeeper, distr.DefaultCodespace, auth.FeeCollectorName, app.ModuleAccountAddrs())

	app.slashingKeeper = slashing.NewKeeper(app.cdc, keys[slashing.StoreKey], &stakingKeeper,
		slashingSubspace, slashing.DefaultCodespace)

	// register the staking hooks
	// NOTE: stakingKeeper above is passed by reference, so that it will contain these hooks
	app.stakingKeeper = *stakingKeeper.SetHooks(
		staking.NewMultiStakingHooks(app.distrKeeper.Hooks(), app.slashingKeeper.Hooks()),
	)

	// * Set up a new keeper with storage, code, and block keys
	app.evmKeeper = evm.NewKeeper(app.accountKeeper, keys[evmtypes.EvmStoreKey],
		keys[evmtypes.EvmCodeKey], blockKey, cdc)

	app.mm = module.NewManager(
		genutil.NewAppModule(app.accountKeeper, app.stakingKeeper, app.BaseApp.DeliverTx),
		auth.NewAppModule(app.accountKeeper),
		bank.NewAppModule(app.bankKeeper, app.accountKeeper),
		supply.NewAppModule(app.supplyKeeper, app.accountKeeper),
		distr.NewAppModule(app.distrKeeper, app.supplyKeeper),
		slashing.NewAppModule(app.slashingKeeper, app.stakingKeeper),
		staking.NewAppModule(app.stakingKeeper, app.accountKeeper, app.supplyKeeper),
		// * Include evm module in module manager
		evm.NewAppModule(app.evmKeeper),
	)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	// * Include EVM module in begin blockers for now (only used for web3 API)
	app.mm.SetOrderBeginBlockers(evmtypes.ModuleName, slashing.ModuleName)

	// * EVM module needs to have end blocker, since state transitions are committed at this point
	app.mm.SetOrderEndBlockers(evmtypes.ModuleName, staking.ModuleName)

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	app.mm.SetOrderInitGenesis(
		distr.ModuleName,
		staking.ModuleName,
		auth.ModuleName,
		bank.ModuleName,
		slashing.ModuleName,
		supply.ModuleName,
		genutil.ModuleName,
		// * The evm module needs to be added here
		evmtypes.ModuleName,
	)

	app.mm.RegisterRoutes(app.Router(), app.QueryRouter())

	// initialize BaseApp
	app.SetInitChainer(app.InitChainer)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)

	// Base ante handler
	app.SetAnteHandler(
		auth.NewAnteHandler(
			app.accountKeeper,
			app.supplyKeeper,
			auth.DefaultSigVerificationGasConsumer,
		),
	)
	// initialize stores
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	// * Mount block hash storage key here as db (again, only used for web3 API)
	app.MountStore(blockKey, sdk.StoreTypeDB)

	if loadLatest {
		err := app.LoadLatestVersion(app.keys[bam.MainStoreKey])
		if err != nil {
			cmn.Exit(err.Error())
		}
	}
	return app
}

// GenesisState is the state of the blockchain is represented here as a map of raw json
// messages key'd by a identifier string.
type GenesisState map[string]json.RawMessage

// BeginBlocker updates every begin block
func (app *EthermintyApp) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	return app.mm.BeginBlock(ctx, req)
}

// EndBlocker updates every end block
func (app *EthermintyApp) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	return app.mm.EndBlock(ctx, req)
}

// InitChainer updates at chain initialization
func (app *EthermintyApp) InitChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	var genesisState GenesisState

	err := app.cdc.UnmarshalJSON(req.AppStateBytes, &genesisState)
	if err != nil {
		panic(err)
	}

	return app.mm.InitGenesis(ctx, genesisState)
}

// LoadHeight loads state at a particular height
func (app *EthermintyApp) LoadHeight(height int64) error {
	return app.LoadVersion(height, app.keys[bam.MainStoreKey])
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *EthermintyApp) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[app.supplyKeeper.GetModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// ExportAppStateAndValidators exports the state of the application for a genesis
// file.
func (app *EthermintyApp) ExportAppStateAndValidators(forZeroHeight bool, jailWhiteList []string,
) (appState json.RawMessage, validators []tmtypes.GenesisValidator, err error) {

	// as if they could withdraw from the start of the next block
	ctx := app.NewContext(true, abci.Header{Height: app.LastBlockHeight()})

	genState := app.mm.ExportGenesis(ctx)
	appState, err = codec.MarshalJSONIndent(app.cdc, genState)
	if err != nil {
		return nil, nil, err
	}

	validators = staking.WriteValidators(ctx, app.stakingKeeper)

	return appState, validators, nil
}
