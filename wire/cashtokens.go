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
)

type TokenData struct {
	CategoryID [32]byte
	Commitment []byte
	Amount     uint64
	BitField   byte
}

func (tokenData *TokenData) SeparateTokenDataFromPKScriptIfExists(buf []byte, pver uint32) ([]byte, error) {
	if len(buf) == 0 || buf[0] != PREFIX_BYTE {
		// There is no token data. Return the whole buffer as script
		return buf, nil
	} else {
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

		if tokenData.IsValidBitfield() { // Raise error if false?

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

				if amount >= 1 && amount <= 9223372036854775807 {
					tokenData.Amount = amount
				} else {
					return nil, errors.New("invalid amount")
				}

				scriptLengthCount -= 1
			}
		} else {
			return nil, errors.New("invalid bitfield")
		}

		var pkScript []byte
		//b := scriptPool.Borrow(uint64(scriptLengthCount))
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
	if tokenData.BitField&0xf0 >= 0x80 || tokenData.BitField&0xf0 == 0x00 {
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
		//WriteVarInt(&buf, 0, uint64(tokenData.BitField&HAS_COMMITMENT_LENGTH))

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

	var Genesis_Categories [][32]byte
	Available_Sums_By_Category := make(map[[32]byte]uint64)
	Available_Mutable_Tokens_By_Category := make(map[[32]byte]int64)
	var Input_Minting_Categories [][32]byte
	var Available_Immutable_Tokens []struct {
		category   [32]byte
		commitment []byte
	}
	var Available_Minting_Categories [][32]byte
	for i, txIn := range tx.TxIn {
		utxo, _ := cache.GetEntry(i)

		if txIn.PreviousOutPoint.Index == 0 {
			Genesis_Categories = append(Genesis_Categories, txIn.PreviousOutPoint.Hash)
		}

		if utxo.TokenData.IsEmpty() {
			continue
		}
		value, ok := Available_Sums_By_Category[utxo.TokenData.CategoryID]
		if ok {
			Available_Sums_By_Category[utxo.TokenData.CategoryID] = value + uint64(utxo.TokenData.Amount)
		} else {
			Available_Sums_By_Category[utxo.TokenData.CategoryID] = uint64(utxo.TokenData.Amount)
		}

		if utxo.TokenData.IsMutableNFT() {
			value, ok := Available_Mutable_Tokens_By_Category[utxo.TokenData.CategoryID]
			if ok {
				Available_Mutable_Tokens_By_Category[utxo.TokenData.CategoryID] = value + 1
			} else {
				Available_Mutable_Tokens_By_Category[utxo.TokenData.CategoryID] = 1
			}
		}

		if utxo.TokenData.IsMintingNFT() {
			categoryIDExists := false
			for _, categoryID := range Input_Minting_Categories {
				if categoryID == utxo.TokenData.CategoryID {
					categoryIDExists = true
				}
			}
			if !categoryIDExists {
				Input_Minting_Categories = append(Input_Minting_Categories, utxo.TokenData.CategoryID) // TODO deduplicate it
			}
		}

		if utxo.TokenData.IsImmutableNFT() {
			Available_Immutable_Tokens = append(Available_Immutable_Tokens,
				struct {
					category   [32]byte
					commitment []byte
				}{
					utxo.TokenData.CategoryID, utxo.TokenData.Commitment,
				},
			)
		}
	}
	Available_Minting_Categories = append(Genesis_Categories[:], Input_Minting_Categories...)

	Output_Sums_By_Category := make(map[[32]byte]uint64)
	Output_Mutable_Tokens_By_Category := make(map[[32]byte]int64)
	var Output_Minting_Categories [][32]byte
	var Output_Immutable_Tokens []struct {
		category   [32]byte
		commitment []byte
	}
	for _, txOut := range tx.TxOut {
		if txOut.TokenData.IsEmpty() {
			continue
		}

		if txOut.TokenData.HasAmount() && (txOut.TokenData.Amount < 1 || txOut.TokenData.Amount > 9223372036854775807) {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}
		if len(txOut.TokenData.Commitment) > 40 {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}

		value, ok := Output_Sums_By_Category[txOut.TokenData.CategoryID]
		if ok {
			Output_Sums_By_Category[txOut.TokenData.CategoryID] = value + txOut.TokenData.Amount
		} else {
			Output_Sums_By_Category[txOut.TokenData.CategoryID] = txOut.TokenData.Amount
		}

		if txOut.TokenData.IsMutableNFT() {
			value, ok := Output_Mutable_Tokens_By_Category[txOut.TokenData.CategoryID]
			if ok {
				Output_Mutable_Tokens_By_Category[txOut.TokenData.CategoryID] = value + 1
			} else {
				Output_Mutable_Tokens_By_Category[txOut.TokenData.CategoryID] = 1
			}
		}

		if txOut.TokenData.IsMintingNFT() {
			categoryIDExists := false
			for _, categoryID := range Output_Minting_Categories {
				if categoryID == txOut.TokenData.CategoryID {
					categoryIDExists = true
				}
			}
			if !categoryIDExists {
				Output_Minting_Categories = append(Output_Minting_Categories, txOut.TokenData.CategoryID) // TODO deduplicate it
			}
		}

		if txOut.TokenData.IsImmutableNFT() {
			Output_Immutable_Tokens = append(Output_Immutable_Tokens,
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

	// Each category in Output_Minting_Categories must exist in Available_Minting_Categories.
	categoryIsMissing := true
	for _, category := range Output_Minting_Categories {
		for _, availableCategory := range Available_Minting_Categories {
			if category == availableCategory {
				categoryIsMissing = false
			}
		}
	}
	if len(Output_Minting_Categories) == 0 {
		categoryIsMissing = false
	}
	if categoryIsMissing {
		return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
	}

	// Each category in Output_Sums_By_Category must either:
	//     Have an equal or greater sum in Available_Sums_By_Category, or
	//     Exist in Genesis_Categories and have an output sum no greater than 9223372036854775807 (the maximum VM number).
	for outputCategory, tokenOutputValue := range Output_Sums_By_Category {
		availableSum, ok := Available_Sums_By_Category[outputCategory]
		if !ok || tokenOutputValue > availableSum {
			existsInGenesisCategories := false
			for _, genesisCategory := range Genesis_Categories {
				if genesisCategory == outputCategory {
					existsInGenesisCategories = true
				}
			}
			if !existsInGenesisCategories || tokenOutputValue > 9223372036854775807 {
				return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
			}
		}
	}

	// For each category in Output_Mutable_Tokens_By_Category, if the token's category ID exists in Available_Minting_Categories, skip this (valid) category.
	// Else: Deduct the sum in Output_Mutable_Tokens_By_Category from the sum available in Available_Mutable_Tokens_By_Category.
	// If the value falls below 0, fail validation.
	for outputCategory, tokenOutputValue := range Output_Mutable_Tokens_By_Category {
		existsInAvailableMintingCategories := false
		for _, mintingCategory := range Available_Minting_Categories {
			if mintingCategory == outputCategory {
				existsInAvailableMintingCategories = true
				break
			}
		}
		if !existsInAvailableMintingCategories {
			Available_Mutable_Tokens_By_Category[outputCategory] -= tokenOutputValue
			if Available_Mutable_Tokens_By_Category[outputCategory] < 0 {
				return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
			}
		}
	}

	// For each token in Output_Immutable_Tokens, if the token's category ID exists in Available_Minting_Categories, skip this (valid) token. Else:
	// If an equivalent token exists in Available_Immutable_Tokens (comparing both category ID and commitment), remove it and continue to the next token. Else:
	//     Deduct 1 from the sum available for the token's category in Available_Mutable_Tokens_By_Category. If no mutable tokens are available to downgrade, fail validation.

out:
	for _, outputImmutableToken := range Output_Immutable_Tokens {
		existsInAvailableMintingCategories := false
		for _, mintingCategory := range Available_Minting_Categories {
			if mintingCategory == outputImmutableToken.category {
				existsInAvailableMintingCategories = true
				continue out
			}
		}

		if !existsInAvailableMintingCategories {
			for i, availableImmutableToken := range Available_Immutable_Tokens {
				if availableImmutableToken.category == outputImmutableToken.category &&
					bytes.Equal(availableImmutableToken.commitment, outputImmutableToken.commitment) {
					Available_Immutable_Tokens = append(Available_Immutable_Tokens[:i], Available_Immutable_Tokens[i+1:]...)
					continue out
				}
			}
			_, ok := Available_Mutable_Tokens_By_Category[outputImmutableToken.category]
			if !ok {
				return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
			}
			if ok {
				if Available_Mutable_Tokens_By_Category[outputImmutableToken.category] <= 0 {
					return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
				}
				Available_Mutable_Tokens_By_Category[outputImmutableToken.category] -= 1
				continue out
			}
		}
	}

	return true, nil
}
