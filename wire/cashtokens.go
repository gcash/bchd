package wire

import (
	"bytes"
	"io"
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

	BASE_TOEN_DATA_LENGTH = 1 + 32 + 1
)

type TokenData struct {
	CategoryID [32]byte
	Commitment []byte
	Amount     uint64
	BitField   byte
}

func (tokenData *TokenData) SeparateTokenDataFromPKScriptIfExists(buf []byte, pver uint32) ([]byte, error) {
	if buf[0] != PREFIX_BYTE {
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
				}
			}
			if tokenData.HasAmount() {
				amount, err := ReadVarInt(r, pver)
				if err != nil {
					return nil, err
				}

				if amount >= 1 && amount <= 9223372036854775807 {
					tokenData.Amount = amount
				}

				scriptLengthCount -= 1
			}
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
		WriteVarInt(&buf, 0, uint64(tokenData.BitField&HAS_COMMITMENT_LENGTH))

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
