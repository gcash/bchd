package wire

import (
	"bytes"
	"errors"
	"io"
	"math"
)

const PREFIX_BYTE = 0xef

const (
	NONE    = 0x00
	MUTABLE = 0x01
	MINTING = 0x02

	HAS_AMOUNT            = 0x10
	HAS_NFT               = 0x20
	HAS_COMMITMENT_LENGTH = 0x40
	RESERVED_BIT          = 0x80

	BASE_TOKEN_DATA_LENGTH = 1 + 32 + 1

	MAX_FT_AMOUNT         = 9223372036854775807
	MAX_COMMITMENT_LENGTH = 40
)

type TokenData struct {
	CategoryID [32]byte
	Commitment []byte
	Amount     uint64
	BitField   byte
}

func NewTokenData(categoryID [32]byte, amount *uint64, commitment *[]byte, compability *byte) (*TokenData, error) {
	tokenData := TokenData{}
	tokenData.CategoryID = categoryID
	tokenData.BitField = 0

	if amount != nil {

		if *amount <= 0 || *amount > MAX_FT_AMOUNT {
			return nil, errors.New("invalid token amount")
		}
		tokenData.Amount = *amount

		// bitfield has amount
		tokenData.BitField = tokenData.BitField | HAS_AMOUNT
	}

	if commitment != nil {

		if len(*commitment) > MAX_COMMITMENT_LENGTH {
			return nil, errors.New("invalid token commitment length")
		}

		tokenData.Commitment = *commitment
		// bitfield has commitment length
		tokenData.BitField = tokenData.BitField | HAS_NFT
		tokenData.BitField = tokenData.BitField | HAS_COMMITMENT_LENGTH
	}

	if compability != nil {
		if *compability < NONE || *compability > MINTING {
			return nil, errors.New("invalid token compability")
		}
		// bitfield has nft
		tokenData.BitField = tokenData.BitField | HAS_NFT
		tokenData.BitField = tokenData.BitField | *compability

	}
	isValid := tokenData.IsValidBitfield()
	if !isValid {
		return nil, errors.New("invalid token bitfield")
	}

	return &tokenData, nil
}

func (tokenData *TokenData) SeparateTokenDataFromPKScriptIfExists(buf []byte, pver uint32) ([]byte, error) {
	if len(buf) == 0 || buf[0] != PREFIX_BYTE {
		// There is no token data. Return the whole buffer as script
		return buf, nil
	}

	scriptLengthCount := len(buf)

	r := bytes.NewReader(buf[1:])
	io.ReadFull(r, tokenData.CategoryID[:])
	// tokenData.CategoryID = buf[1:33]
	bitField, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	tokenData.BitField = bitField

	scriptLengthCount -= (1 + 32 + 1) //PREFIX_BYTE + CategoryID + BitField

	if tokenData.IsValidBitfield() {

		if tokenData.HasCommitmentLength() {
			commitmentLength, err := ReadVarInt(r, pver)
			if err != nil {
				return nil, err
			}

			if commitmentLength >= 0x01 && commitmentLength <= 0x28 {
				b := scriptPool.Borrow(commitmentLength)
				_, err := io.ReadFull(r, b)
				if err != nil {
					scriptPool.Return(b)
					return nil, err
				}
				tokenData.Commitment = b
				scriptLengthCount -= (1 + len(b)) // commitmentLength
			} else {
				return nil, errors.New("invalid commitment length")
			}
		}
		if tokenData.HasAmount() {
			amount, err := ReadVarInt(r, pver)
			if err != nil {
				return nil, err
			}

			if amount >= 1 && amount <= MAX_FT_AMOUNT {
				tokenData.Amount = amount
			} else {
				return nil, errors.New("invalid token amount")
			}

			scriptLengthCount -= 1
		}
	} else {
		return nil, errors.New("invalid bitfield")
	}

	var pkScript []byte
	b := scriptPool.Borrow(uint64(r.Len()))
	_, err = io.ReadFull(r, b)
	if err != nil {
		scriptPool.Return(b)
		return nil, err
	}
	pkScript = b

	//fmt.Println("length of pkscript: ", len(pkScript), tokenData.Commitment)
	return pkScript, nil
}

func (tokenData *TokenData) IsEmpty() bool {
	return tokenData.CategoryID == TokenData{}.CategoryID // TODO maybe change this
}

func (tokenData *TokenData) GetCapability() byte {
	return tokenData.BitField & 0x0f
}

func (tokenData *TokenData) HasCommitmentLength() bool {
	return tokenData.BitField&HAS_COMMITMENT_LENGTH > 0
}

func (tokenData *TokenData) HasAmount() bool {
	return tokenData.BitField&HAS_AMOUNT > 0
}

func (tokenData *TokenData) HasNFT() bool {
	return tokenData.BitField&HAS_NFT > 0
}

func (tokenData *TokenData) IsMintingNFT() bool {
	return tokenData.HasNFT() && tokenData.GetCapability() == MINTING
}

func (tokenData *TokenData) IsMutableNFT() bool {
	return tokenData.HasNFT() && tokenData.GetCapability() == MUTABLE
}

func (tokenData *TokenData) IsImmutableNFT() bool {
	return tokenData.HasNFT() && tokenData.GetCapability() == NONE
}

func (tokenData *TokenData) IsValidBitfield() bool {
	if tokenData.BitField&0xf0 >= RESERVED_BIT || tokenData.BitField&0xf0 == 0x00 {
		return false
	}
	if tokenData.BitField&0x0f > 0x02 {
		return false
	}

	if !tokenData.HasNFT() && !tokenData.HasAmount() {
		return false
	}
	if !tokenData.HasNFT() && (tokenData.BitField&0x0f) != 0 {
		return false
	}
	if !tokenData.HasNFT() && tokenData.HasCommitmentLength() {
		return false
	}

	return true
}

func (tokenData *TokenData) TokenDataBuffer() bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteByte(0xef)
	buf.Write(tokenData.CategoryID[:])
	buf.WriteByte(tokenData.BitField)
	if tokenData.HasCommitmentLength() {
		WriteVarInt(&buf, 0, uint64(len(tokenData.Commitment)))

		buf.Write(tokenData.Commitment[:])
	}
	if tokenData.HasAmount() {
		WriteVarInt(&buf, 0, tokenData.Amount)
	} else {
		var buf2 bytes.Buffer
		buf2.Write(tokenData.CategoryID[:])
	}
	return buf
}

type utxoCacheInterface interface {

	// AddEntry adds a utxo entry for the given input index.
	AddEntry(i int, output TxOut)

	// GetEntry adds a utxo entry for the given input index.
	GetEntry(i int) (TxOut, error)
}

// TODO TODO TODO move this somewhere else or use the original function
func IsCoinBaseTx(msgTx *MsgTx) bool {
	// A coin base must only have one transaction input.
	if len(msgTx.TxIn) != 1 {
		return false
	}

	zeroHash := [32]byte{}
	// The previous output of a coin base must have a max value index and
	// a zero hash.
	prevOut := &msgTx.TxIn[0].PreviousOutPoint
	if prevOut.Index != math.MaxUint32 || prevOut.Hash != zeroHash {
		return false
	}

	return true
}

// Token Validation Algorithm
func RunCashTokensValidityAlgorithm(cache utxoCacheInterface, tx *MsgTx) (bool, error) {
	if IsCoinBaseTx(tx) {
		return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
	}

	var GenesisCategories [][32]byte
	AvailableSumsByCategory := make(map[[32]byte]uint64)
	AvailableMutableTokensByCategory := make(map[[32]byte]int64)
	var InputMintingCategories [][32]byte
	var AvailableImmutableTokens []struct {
		category   [32]byte
		commitment []byte
	}
	var AvailableMintingCategories [][32]byte
	for i, txIn := range tx.TxIn {
		utxo, _ := cache.GetEntry(i)

		if txIn.PreviousOutPoint.Index == 0 {
			GenesisCategories = append(GenesisCategories, txIn.PreviousOutPoint.Hash)
		}

		if utxo.TokenData.IsEmpty() {
			continue
		}
		value, ok := AvailableSumsByCategory[utxo.TokenData.CategoryID]
		if ok {
			AvailableSumsByCategory[utxo.TokenData.CategoryID] = value + utxo.TokenData.Amount
		} else {
			AvailableSumsByCategory[utxo.TokenData.CategoryID] = utxo.TokenData.Amount
		}

		if utxo.TokenData.IsMutableNFT() {
			value, ok := AvailableMutableTokensByCategory[utxo.TokenData.CategoryID]
			if ok {
				AvailableMutableTokensByCategory[utxo.TokenData.CategoryID] = value + 1
			} else {
				AvailableMutableTokensByCategory[utxo.TokenData.CategoryID] = 1
			}
		}

		if utxo.TokenData.IsMintingNFT() {
			categoryIDExists := false
			for _, categoryID := range InputMintingCategories {
				if categoryID == utxo.TokenData.CategoryID {
					categoryIDExists = true
				}
			}
			if !categoryIDExists {
				InputMintingCategories = append(InputMintingCategories, utxo.TokenData.CategoryID) // TODO deduplicate it
			}
		}

		if utxo.TokenData.IsImmutableNFT() {
			AvailableImmutableTokens = append(AvailableImmutableTokens,
				struct {
					category   [32]byte
					commitment []byte
				}{
					utxo.TokenData.CategoryID, utxo.TokenData.Commitment,
				},
			)
		}
	}
	AvailableMintingCategories = append(GenesisCategories[:], InputMintingCategories...)

	OutputSumsByCategory := make(map[[32]byte]uint64)
	OutputMutableTokensByCategory := make(map[[32]byte]int64)
	var OutputMintingCategories [][32]byte
	var OutputImmutableTokens []struct {
		category   [32]byte
		commitment []byte
	}
	for _, txOut := range tx.TxOut {
		if txOut.TokenData.IsEmpty() {
			continue
		}

		if txOut.TokenData.HasAmount() && (txOut.TokenData.Amount < 1 || txOut.TokenData.Amount > MAX_FT_AMOUNT) {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}
		if len(txOut.TokenData.Commitment) > MAX_COMMITMENT_LENGTH {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}

		value, ok := OutputSumsByCategory[txOut.TokenData.CategoryID]
		if ok {
			OutputSumsByCategory[txOut.TokenData.CategoryID] = value + txOut.TokenData.Amount
		} else {
			OutputSumsByCategory[txOut.TokenData.CategoryID] = txOut.TokenData.Amount
		}

		if txOut.TokenData.IsMutableNFT() {
			value, ok := OutputMutableTokensByCategory[txOut.TokenData.CategoryID]
			if ok {
				OutputMutableTokensByCategory[txOut.TokenData.CategoryID] = value + 1
			} else {
				OutputMutableTokensByCategory[txOut.TokenData.CategoryID] = 1
			}
		}

		if txOut.TokenData.IsMintingNFT() {
			categoryIDExists := false
			for _, categoryID := range OutputMintingCategories {
				if categoryID == txOut.TokenData.CategoryID {
					categoryIDExists = true
				}
			}
			if !categoryIDExists {
				OutputMintingCategories = append(OutputMintingCategories, txOut.TokenData.CategoryID) // TODO deduplicate it
			}
		}

		if txOut.TokenData.IsImmutableNFT() {
			OutputImmutableTokens = append(OutputImmutableTokens,
				struct {
					category   [32]byte
					commitment []byte
				}{
					txOut.TokenData.CategoryID, txOut.TokenData.Commitment,
				},
			)
		}
	}

	// run checks

	// Each category in OutputMintingCategories must exist in Available_Minting_Categories.
	categoryIsMissing := true
	for _, category := range OutputMintingCategories {
		for _, availableCategory := range AvailableMintingCategories {
			if category == availableCategory {
				categoryIsMissing = false
			}
		}
	}
	if len(OutputMintingCategories) == 0 {
		categoryIsMissing = false
	}
	if categoryIsMissing {
		return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
	}

	// Each category in OutputSumsByCategory must either:
	//     Have an equal or greater sum in Available_Sums_By_Category, or
	//     Exist in Genesis_Categories and have an output sum no greater than 9223372036854775807 (the maximum VM number).
	for outputCategory, tokenOutputValue := range OutputSumsByCategory {
		availableSum, ok := AvailableSumsByCategory[outputCategory]
		if !ok || tokenOutputValue > availableSum {
			existsInGenesisCategories := false
			for _, genesisCategory := range GenesisCategories {
				if genesisCategory == outputCategory {
					existsInGenesisCategories = true
				}
			}
			if !existsInGenesisCategories || tokenOutputValue > MAX_FT_AMOUNT {
				return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
			}
		}
	}

	// For each category in OutputMutableTokensByCategory, if the token's category ID exists in Available_Minting_Categories, skip this (valid) category.
	// Else: Deduct the sum in OutputMutableTokensByCategory from the sum available in Available_Mutable_Tokens_By_Category.
	// If the value falls below 0, fail validation.
	for outputCategory, tokenOutputValue := range OutputMutableTokensByCategory {
		existsInAvailableMintingCategories := false
		for _, mintingCategory := range AvailableMintingCategories {
			if mintingCategory == outputCategory {
				existsInAvailableMintingCategories = true
				break
			}
		}
		if !existsInAvailableMintingCategories {
			AvailableMutableTokensByCategory[outputCategory] -= tokenOutputValue
			if AvailableMutableTokensByCategory[outputCategory] < 0 {
				return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
			}
		}
	}

	// For each token in OutputImmutableTokens, if the token's category ID exists in Available_Minting_Categories, skip this (valid) token. Else:
	// If an equivalent token exists in Available_Immutable_Tokens (comparing both category ID and commitment), remove it and continue to the next token. Else:
	//     Deduct 1 from the sum available for the token's category in Available_Mutable_Tokens_By_Category. If no mutable tokens are available to downgrade, fail validation.

out:
	for _, outputImmutableToken := range OutputImmutableTokens {
		existsInAvailableMintingCategories := false
		for _, mintingCategory := range AvailableMintingCategories {
			if mintingCategory == outputImmutableToken.category {
				existsInAvailableMintingCategories = true
				continue out
			}
		}

		if !existsInAvailableMintingCategories {
			for i, availableImmutableToken := range AvailableImmutableTokens {
				if availableImmutableToken.category == outputImmutableToken.category &&
					bytes.Equal(availableImmutableToken.commitment, outputImmutableToken.commitment) {
					AvailableImmutableTokens = append(AvailableImmutableTokens[:i], AvailableImmutableTokens[i+1:]...)
					continue out
				}
			}
			_, ok := AvailableMutableTokensByCategory[outputImmutableToken.category]
			if !ok {
				return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
			}
			if ok {
				if AvailableMutableTokensByCategory[outputImmutableToken.category] <= 0 {
					return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
				}
				AvailableMutableTokensByCategory[outputImmutableToken.category] -= 1
				continue out
			}
		}
	}

	return true, nil
}
