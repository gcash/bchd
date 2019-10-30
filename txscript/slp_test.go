package txscript

import "testing"

func TestGetSLPScriptClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		script        []byte
		class         ScriptClass
		expectedClass ScriptClass
		valid         bool
	}{
		{
			"spend, two outputs",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0800000003dd6fe600" +
				"080000001359255f00"),
			SLPSpendTy,
			SLPSpendTy,
			true,
		},
		{
			"spend missing elements",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"spend wrong first opcode",
			hexToBytes("6b" +
				"04534c5000" +
				"0101" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0800000003dd6fe600" +
				"080000001359255f00"),
			SLPSpendTy,
			NonStandardTy,
			false,
		},
		{
			"spend wrong lokad id",
			hexToBytes("6a" +
				"04534c5001" +
				"0101" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0800000003dd6fe600" +
				"080000001359255f00"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"spend invalid push",
			hexToBytes("6a" +
				"04534c5000" +
				"51" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0800000003dd6fe600" +
				"080000001359255f00"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"spend invalid type",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0453454e43" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0800000003dd6fe600" +
				"080000001359255f00"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"spend invalid token type",
			hexToBytes("6a" +
				"04534c5000" +
				"4c00" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0800000003dd6fe600" +
				"080000001359255f00"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"spend invalid token type",
			hexToBytes("6a" +
				"04534c5000" +
				"03010101" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0800000003dd6fe600" +
				"080000001359255f00"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"spend invalid token ID",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0453454e44" +
				"1f4de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0d" +
				"0800000003dd6fe600" +
				"080000001359255f00"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"spend invalid quantity2",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0700000003dd6fe6" +
				"080000001359255f00"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"spend invalid additional quantity",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0453454e44" +
				"204de69e374a8ed21cbddd47f2338cc0f479dc58daa2bbe11cd604ca488eca0ddf" +
				"0800000003dd6fe600" +
				"070000001359255f"),
			SLPSpendTy,
			NullDataTy,
			false,
		},
		{
			"genesis",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			SLPGenesisTy,
			true,
		},
		{
			"genesis invalid lokad ID",
			hexToBytes("6a" +
				"04534c5001" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis incorrect first opcode",
			hexToBytes("6b" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NonStandardTy,
			false,
		},
		{
			"genesis incorrect number of elements",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid transaction type",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534952" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid push",
			hexToBytes("6a" +
				"04534c5000" +
				"51" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid token type",
			hexToBytes("6a" +
				"04534c5000" +
				"4c00" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid token type 2",
			hexToBytes("6a" +
				"04534c5000" +
				"03010101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid token hash",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"0101" +
				"0108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid decimals",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0110" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid decimals 2",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"020108" +
				"4c00" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid decimals 2",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"020101" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid decimals 2",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"0101" +
				"08016345785d8a0000"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"genesis invalid quantity",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"0747454e45534953" +
				"055350494345" +
				"055370696365" +
				"127370696365736c7040676d61696c2e636f6d" +
				"4c00" +
				"0108" +
				"4c00" +
				"09016345785d8a000011"),
			SLPGenesisTy,
			NullDataTy,
			false,
		},
		{
			"mint",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0102" +
				"080000000000000064"),
			SLPMintTy,
			SLPMintTy,
			true,
		},
		{
			"mint invalid push",
			hexToBytes("6a" +
				"04534c5000" +
				"51" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0102" +
				"080000000000000064"),
			SLPMintTy,
			NullDataTy,
			false,
		},
		{
			"mint invalid first opcode",
			hexToBytes("6b" +
				"04534c5000" +
				"0101" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0102" +
				"080000000000000064"),
			SLPMintTy,
			NonStandardTy,
			false,
		},
		{
			"mint invalid transaction type",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"044d494e55" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0102" +
				"080000000000000064"),
			SLPMintTy,
			NullDataTy,
			false,
		},
		{
			"mint invalid lokad ID",
			hexToBytes("6a" +
				"04534c5001" +
				"0101" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0102" +
				"080000000000000064"),
			SLPMintTy,
			NullDataTy,
			false,
		},
		{
			"mint invalid token type",
			hexToBytes("6a" +
				"04534c5000" +
				"4c00" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0102" +
				"080000000000000064"),
			SLPMintTy,
			NullDataTy,
			false,
		},
		{
			"mint invalid token type 2",
			hexToBytes("6a" +
				"04534c5000" +
				"03010101" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0102" +
				"080000000000000064"),
			SLPMintTy,
			NullDataTy,
			false,
		},
		{
			"mint invalid token ID",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"044d494e54" +
				"213101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a00" +
				"0102" +
				"080000000000000064"),
			SLPMintTy,
			NullDataTy,
			false,
		},
		{
			"mint invalid baton vout",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"020101" +
				"080000000000000064"),
			SLPMintTy,
			NullDataTy,
			false,
		},
		{
			"mint invalid baton vout 2",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0101" +
				"080000000000000064"),
			SLPMintTy,
			NullDataTy,
			false,
		},
		{
			"mint invalid quantity",
			hexToBytes("6a" +
				"04534c5000" +
				"0101" +
				"044d494e54" +
				"203101f6b05e0f0d7cea649864846a42aea2cbd6ae5989692f32a818842200437a" +
				"0102" +
				"090000000000000064ff"),
			SLPMintTy,
			NullDataTy,
			false,
		},
	}

	for _, test := range tests {
		pops, err := parseScript(test.script)
		if err != nil {
			t.Errorf("%s: parse script error: %s", test.name, err)
			continue
		}
		var valid bool
		switch test.class {
		case SLPGenesisTy:
			valid = isSLPGenesis(pops)
		case SLPMintTy:
			valid = isSLPMint(pops)
		case SLPSpendTy:
			valid = isSLPSpend(pops)
		}

		if valid != test.valid {
			t.Errorf("%s: expected %t, got %t", test.name, test.valid, valid)
		}

		class := GetSLPScriptClass(test.script)
		if class != test.expectedClass {
			t.Errorf("%s: expected %s, got %s", test.name, test.expectedClass, class)
		}
	}
}
