package wire

import (
	"bytes"
	"errors"
	"io"
	"math"
	"slices"
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

	MAX_FT_AMOUNT = 9223372036854775807

	MAX_COMMITMENT_LENGTH_LEGACY = 40
	MAX_COMMITMENT_LENGTH        = 128
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

// SeparateTokenDataFromPKScriptIfExists splits a serialized output script into
// its token data and locking script.  A script that does not start with
// PREFIX_BYTE is returned unchanged with empty token data.  An error is
// returned, and the token data is left empty, when the script starts with
// PREFIX_BYTE but does not hold a valid token prefix.
func (tokenData *TokenData) SeparateTokenDataFromPKScriptIfExists(buf []byte, pver uint32) ([]byte, error) {
	*tokenData = TokenData{}
	if len(buf) == 0 || buf[0] != PREFIX_BYTE {
		return buf, nil
	}

	parsed, prefixLen, err := parseTokenPrefix(buf, pver)
	if err != nil {
		return nil, err
	}
	*tokenData = parsed
	return buf[prefixLen:], nil
}

// SplitPkScript splits a serialized output script into its token data and
// locking script.  A script without a valid token prefix is returned whole
// with empty token data, the same way outputs are decoded.
func SplitPkScript(buf []byte) (TokenData, []byte) {
	if len(buf) > 0 && buf[0] == PREFIX_BYTE {
		tokenData, prefixLen, err := parseTokenPrefix(buf, 0)
		if err == nil {
			return tokenData, buf[prefixLen:]
		}
	}
	return TokenData{}, buf
}

// parseTokenPrefix parses the token prefix at the start of buf and returns the
// token data along with the length of the prefix.
func parseTokenPrefix(buf []byte, pver uint32) (TokenData, int, error) {
	var tokenData TokenData
	if len(buf) < BASE_TOKEN_DATA_LENGTH || buf[0] != PREFIX_BYTE {
		return TokenData{}, 0, errors.New("token prefix too short")
	}
	copy(tokenData.CategoryID[:], buf[1:33])
	tokenData.BitField = buf[33]
	if !tokenData.IsValidBitfield() {
		return TokenData{}, 0, errors.New("invalid bitfield")
	}

	r := bytes.NewReader(buf[BASE_TOKEN_DATA_LENGTH:])
	if tokenData.HasCommitmentLength() {
		commitmentLength, err := ReadVarInt(r, pver)
		if err != nil {
			return TokenData{}, 0, err
		}

		// The commitment is copied so it doesn't keep buf alive or
		// alias it, since buf may be returned to the script pool.
		if commitmentLength < 1 || commitmentLength > MAX_COMMITMENT_LENGTH ||
			commitmentLength > uint64(r.Len()) {
			return TokenData{}, 0, errors.New("invalid commitment length")
		}
		start := len(buf) - r.Len()
		end := start + int(commitmentLength)
		tokenData.Commitment = slices.Clone(buf[start:end])
		if _, err := r.Seek(int64(commitmentLength), io.SeekCurrent); err != nil {
			return TokenData{}, 0, err
		}
	}
	if tokenData.HasAmount() {
		amount, err := ReadVarInt(r, pver)
		if err != nil {
			return TokenData{}, 0, err
		}
		if amount < 1 || amount > MAX_FT_AMOUNT {
			return TokenData{}, 0, errors.New("invalid token amount")
		}
		tokenData.Amount = amount
	}

	return tokenData, len(buf) - r.Len(), nil
}

// IsEmpty returns whether there is no token.  A parsed or constructed token
// always has a valid, and therefore non-zero, bitfield.
func (tokenData *TokenData) IsEmpty() bool {
	return tokenData.BitField == 0
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
		_ = WriteVarInt(&buf, 0, uint64(len(tokenData.Commitment)))

		buf.Write(tokenData.Commitment[:])
	}
	if tokenData.HasAmount() {
		_ = WriteVarInt(&buf, 0, tokenData.Amount)
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

// hasInvalidTokenPrefix returns whether the output script starts with the
// token prefix byte without holding a valid token prefix.
func hasInvalidTokenPrefix(txOut *TxOut) bool {
	return txOut.TokenData.IsEmpty() && len(txOut.PkScript) > 0 &&
		txOut.PkScript[0] == PREFIX_BYTE
}

// Token Validation Algorithm
func RunCashTokensValidityAlgorithm(cache utxoCacheInterface, tx *MsgTx, hasScriptAllowMay2026 bool) (bool, error) {
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

	maxCommitmentLength := MAX_COMMITMENT_LENGTH_LEGACY
	if hasScriptAllowMay2026 {
		maxCommitmentLength = MAX_COMMITMENT_LENGTH
	}

	for i, txIn := range tx.TxIn {
		utxo, _ := cache.GetEntry(i)

		if hasInvalidTokenPrefix(&utxo) {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}

		if txIn.PreviousOutPoint.Index == 0 {
			GenesisCategories = append(GenesisCategories, txIn.PreviousOutPoint.Hash)
		}

		if utxo.TokenData.IsEmpty() {
			continue
		}

		if len(utxo.TokenData.Commitment) > maxCommitmentLength {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}

		// Reject a sum above MAX_FT_AMOUNT rather than let it wrap.  Each
		// amount is at most MAX_FT_AMOUNT, so the addition cannot overflow.
		availableSum := AvailableSumsByCategory[utxo.TokenData.CategoryID] + utxo.TokenData.Amount
		if availableSum > MAX_FT_AMOUNT {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}
		AvailableSumsByCategory[utxo.TokenData.CategoryID] = availableSum

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
	// Use a fresh slice rather than appending onto GenesisCategories, which
	// would share its backing array.
	AvailableMintingCategories = slices.Concat(GenesisCategories, InputMintingCategories)

	OutputSumsByCategory := make(map[[32]byte]uint64)
	OutputMutableTokensByCategory := make(map[[32]byte]int64)
	var OutputMintingCategories [][32]byte
	var OutputImmutableTokens []struct {
		category   [32]byte
		commitment []byte
	}

	for _, txOut := range tx.TxOut {
		if hasInvalidTokenPrefix(txOut) {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}

		if txOut.TokenData.IsEmpty() {
			continue
		}

		if !txOut.TokenData.HasNFT() && (txOut.TokenData.Amount < 1 || txOut.TokenData.Amount > MAX_FT_AMOUNT) {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}
		if len(txOut.TokenData.Commitment) > maxCommitmentLength {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}

		// Reject a sum above MAX_FT_AMOUNT, which could otherwise wrap and
		// pass the checks below.
		outputSum := OutputSumsByCategory[txOut.TokenData.CategoryID] + txOut.TokenData.Amount
		if outputSum > MAX_FT_AMOUNT {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}
		OutputSumsByCategory[txOut.TokenData.CategoryID] = outputSum

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
	// Note this must be evaluated per category: a single authorized minting
	// category must not authorize the remaining ones, otherwise a transaction
	// could mint an NFT for a category it holds no minting right to.
	for _, category := range OutputMintingCategories {
		if !slices.Contains(AvailableMintingCategories, category) {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}
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
		if !slices.Contains(AvailableMintingCategories, outputCategory) {
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
		// A category the transaction is allowed to mint can create any
		// immutable token of that category without consuming anything.
		if slices.Contains(AvailableMintingCategories, outputImmutableToken.category) {
			continue
		}

		// Otherwise consume an equivalent available immutable token if one
		// exists.
		for i, availableImmutableToken := range AvailableImmutableTokens {
			if availableImmutableToken.category == outputImmutableToken.category &&
				bytes.Equal(availableImmutableToken.commitment, outputImmutableToken.commitment) {
				AvailableImmutableTokens = append(AvailableImmutableTokens[:i], AvailableImmutableTokens[i+1:]...)
				continue out
			}
		}

		// Failing that, downgrade an available mutable token of the same
		// category.  A missing category reads as zero, which correctly fails
		// this check.
		if AvailableMutableTokensByCategory[outputImmutableToken.category] <= 0 {
			return false, messageError("RunCashTokensValidityAlgorithm", "ErrCashTokensValidation")
		}
		AvailableMutableTokensByCategory[outputImmutableToken.category] -= 1
	}

	return true, nil
}
