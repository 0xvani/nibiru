package dex_test

import (
	"testing"

	"github.com/MatrixDao/matrix/x/dex"
	"github.com/MatrixDao/matrix/x/dex/types"
	"github.com/MatrixDao/matrix/x/testutil"
	"github.com/MatrixDao/matrix/x/testutil/nullify"
	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	app, ctx := testutil.NewMatrixApp()
	dex.InitGenesis(ctx, app.DexKeeper, genesisState)
	got := dex.ExportGenesis(ctx, app.DexKeeper)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	require.Equal(t, genesisState, *got)
}