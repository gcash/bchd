// package: pb
// file: bchrpc.proto

import * as jspb from "google-protobuf";

export class GetMempoolInfoRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetMempoolInfoRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetMempoolInfoRequest): GetMempoolInfoRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetMempoolInfoRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetMempoolInfoRequest;
  static deserializeBinaryFromReader(message: GetMempoolInfoRequest, reader: jspb.BinaryReader): GetMempoolInfoRequest;
}

export namespace GetMempoolInfoRequest {
  export type AsObject = {
  }
}

export class GetMempoolInfoResponse extends jspb.Message {
  getSize(): number;
  setSize(value: number): void;

  getBytes(): number;
  setBytes(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetMempoolInfoResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetMempoolInfoResponse): GetMempoolInfoResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetMempoolInfoResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetMempoolInfoResponse;
  static deserializeBinaryFromReader(message: GetMempoolInfoResponse, reader: jspb.BinaryReader): GetMempoolInfoResponse;
}

export namespace GetMempoolInfoResponse {
  export type AsObject = {
    size: number,
    bytes: number,
  }
}

export class GetMempoolRequest extends jspb.Message {
  getFullTransactions(): boolean;
  setFullTransactions(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetMempoolRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetMempoolRequest): GetMempoolRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetMempoolRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetMempoolRequest;
  static deserializeBinaryFromReader(message: GetMempoolRequest, reader: jspb.BinaryReader): GetMempoolRequest;
}

export namespace GetMempoolRequest {
  export type AsObject = {
    fullTransactions: boolean,
  }
}

export class GetMempoolResponse extends jspb.Message {
  clearTransactionDataList(): void;
  getTransactionDataList(): Array<GetMempoolResponse.TransactionData>;
  setTransactionDataList(value: Array<GetMempoolResponse.TransactionData>): void;
  addTransactionData(value?: GetMempoolResponse.TransactionData, index?: number): GetMempoolResponse.TransactionData;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetMempoolResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetMempoolResponse): GetMempoolResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetMempoolResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetMempoolResponse;
  static deserializeBinaryFromReader(message: GetMempoolResponse, reader: jspb.BinaryReader): GetMempoolResponse;
}

export namespace GetMempoolResponse {
  export type AsObject = {
    transactionDataList: Array<GetMempoolResponse.TransactionData.AsObject>,
  }

  export class TransactionData extends jspb.Message {
    hasTransactionHash(): boolean;
    clearTransactionHash(): void;
    getTransactionHash(): Uint8Array | string;
    getTransactionHash_asU8(): Uint8Array;
    getTransactionHash_asB64(): string;
    setTransactionHash(value: Uint8Array | string): void;

    hasTransaction(): boolean;
    clearTransaction(): void;
    getTransaction(): Transaction | undefined;
    setTransaction(value?: Transaction): void;

    getTxidsOrTxsCase(): TransactionData.TxidsOrTxsCase;
    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TransactionData.AsObject;
    static toObject(includeInstance: boolean, msg: TransactionData): TransactionData.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TransactionData, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TransactionData;
    static deserializeBinaryFromReader(message: TransactionData, reader: jspb.BinaryReader): TransactionData;
  }

  export namespace TransactionData {
    export type AsObject = {
      transactionHash: Uint8Array | string,
      transaction?: Transaction.AsObject,
    }

    export enum TxidsOrTxsCase {
      TXIDS_OR_TXS_NOT_SET = 0,
      TRANSACTION_HASH = 1,
      TRANSACTION = 2,
    }
  }
}

export class GetBlockchainInfoRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBlockchainInfoRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetBlockchainInfoRequest): GetBlockchainInfoRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBlockchainInfoRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBlockchainInfoRequest;
  static deserializeBinaryFromReader(message: GetBlockchainInfoRequest, reader: jspb.BinaryReader): GetBlockchainInfoRequest;
}

export namespace GetBlockchainInfoRequest {
  export type AsObject = {
  }
}

export class GetBlockchainInfoResponse extends jspb.Message {
  getBitcoinNet(): GetBlockchainInfoResponse.BitcoinNetMap[keyof GetBlockchainInfoResponse.BitcoinNetMap];
  setBitcoinNet(value: GetBlockchainInfoResponse.BitcoinNetMap[keyof GetBlockchainInfoResponse.BitcoinNetMap]): void;

  getBestHeight(): number;
  setBestHeight(value: number): void;

  getBestBlockHash(): Uint8Array | string;
  getBestBlockHash_asU8(): Uint8Array;
  getBestBlockHash_asB64(): string;
  setBestBlockHash(value: Uint8Array | string): void;

  getDifficulty(): number;
  setDifficulty(value: number): void;

  getMedianTime(): number;
  setMedianTime(value: number): void;

  getTxIndex(): boolean;
  setTxIndex(value: boolean): void;

  getAddrIndex(): boolean;
  setAddrIndex(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBlockchainInfoResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetBlockchainInfoResponse): GetBlockchainInfoResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBlockchainInfoResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBlockchainInfoResponse;
  static deserializeBinaryFromReader(message: GetBlockchainInfoResponse, reader: jspb.BinaryReader): GetBlockchainInfoResponse;
}

export namespace GetBlockchainInfoResponse {
  export type AsObject = {
    bitcoinNet: GetBlockchainInfoResponse.BitcoinNetMap[keyof GetBlockchainInfoResponse.BitcoinNetMap],
    bestHeight: number,
    bestBlockHash: Uint8Array | string,
    difficulty: number,
    medianTime: number,
    txIndex: boolean,
    addrIndex: boolean,
  }

  export interface BitcoinNetMap {
    MAINNET: 0;
    REGTEST: 1;
    TESTNET3: 2;
    SIMNET: 3;
  }

  export const BitcoinNet: BitcoinNetMap;
}

export class GetBlockInfoRequest extends jspb.Message {
  hasHash(): boolean;
  clearHash(): void;
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  hasHeight(): boolean;
  clearHeight(): void;
  getHeight(): number;
  setHeight(value: number): void;

  getHashOrHeightCase(): GetBlockInfoRequest.HashOrHeightCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBlockInfoRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetBlockInfoRequest): GetBlockInfoRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBlockInfoRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBlockInfoRequest;
  static deserializeBinaryFromReader(message: GetBlockInfoRequest, reader: jspb.BinaryReader): GetBlockInfoRequest;
}

export namespace GetBlockInfoRequest {
  export type AsObject = {
    hash: Uint8Array | string,
    height: number,
  }

  export enum HashOrHeightCase {
    HASH_OR_HEIGHT_NOT_SET = 0,
    HASH = 1,
    HEIGHT = 2,
  }
}

export class GetBlockInfoResponse extends jspb.Message {
  hasInfo(): boolean;
  clearInfo(): void;
  getInfo(): BlockInfo | undefined;
  setInfo(value?: BlockInfo): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBlockInfoResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetBlockInfoResponse): GetBlockInfoResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBlockInfoResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBlockInfoResponse;
  static deserializeBinaryFromReader(message: GetBlockInfoResponse, reader: jspb.BinaryReader): GetBlockInfoResponse;
}

export namespace GetBlockInfoResponse {
  export type AsObject = {
    info?: BlockInfo.AsObject,
  }
}

export class GetBlockRequest extends jspb.Message {
  hasHash(): boolean;
  clearHash(): void;
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  hasHeight(): boolean;
  clearHeight(): void;
  getHeight(): number;
  setHeight(value: number): void;

  getFullTransactions(): boolean;
  setFullTransactions(value: boolean): void;

  getHashOrHeightCase(): GetBlockRequest.HashOrHeightCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBlockRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetBlockRequest): GetBlockRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBlockRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBlockRequest;
  static deserializeBinaryFromReader(message: GetBlockRequest, reader: jspb.BinaryReader): GetBlockRequest;
}

export namespace GetBlockRequest {
  export type AsObject = {
    hash: Uint8Array | string,
    height: number,
    fullTransactions: boolean,
  }

  export enum HashOrHeightCase {
    HASH_OR_HEIGHT_NOT_SET = 0,
    HASH = 1,
    HEIGHT = 2,
  }
}

export class GetBlockResponse extends jspb.Message {
  hasBlock(): boolean;
  clearBlock(): void;
  getBlock(): Block | undefined;
  setBlock(value?: Block): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBlockResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetBlockResponse): GetBlockResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBlockResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBlockResponse;
  static deserializeBinaryFromReader(message: GetBlockResponse, reader: jspb.BinaryReader): GetBlockResponse;
}

export namespace GetBlockResponse {
  export type AsObject = {
    block?: Block.AsObject,
  }
}

export class GetRawBlockRequest extends jspb.Message {
  hasHash(): boolean;
  clearHash(): void;
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  hasHeight(): boolean;
  clearHeight(): void;
  getHeight(): number;
  setHeight(value: number): void;

  getHashOrHeightCase(): GetRawBlockRequest.HashOrHeightCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetRawBlockRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetRawBlockRequest): GetRawBlockRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetRawBlockRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetRawBlockRequest;
  static deserializeBinaryFromReader(message: GetRawBlockRequest, reader: jspb.BinaryReader): GetRawBlockRequest;
}

export namespace GetRawBlockRequest {
  export type AsObject = {
    hash: Uint8Array | string,
    height: number,
  }

  export enum HashOrHeightCase {
    HASH_OR_HEIGHT_NOT_SET = 0,
    HASH = 1,
    HEIGHT = 2,
  }
}

export class GetRawBlockResponse extends jspb.Message {
  getBlock(): Uint8Array | string;
  getBlock_asU8(): Uint8Array;
  getBlock_asB64(): string;
  setBlock(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetRawBlockResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetRawBlockResponse): GetRawBlockResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetRawBlockResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetRawBlockResponse;
  static deserializeBinaryFromReader(message: GetRawBlockResponse, reader: jspb.BinaryReader): GetRawBlockResponse;
}

export namespace GetRawBlockResponse {
  export type AsObject = {
    block: Uint8Array | string,
  }
}

export class GetBlockFilterRequest extends jspb.Message {
  hasHash(): boolean;
  clearHash(): void;
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  hasHeight(): boolean;
  clearHeight(): void;
  getHeight(): number;
  setHeight(value: number): void;

  getHashOrHeightCase(): GetBlockFilterRequest.HashOrHeightCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBlockFilterRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetBlockFilterRequest): GetBlockFilterRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBlockFilterRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBlockFilterRequest;
  static deserializeBinaryFromReader(message: GetBlockFilterRequest, reader: jspb.BinaryReader): GetBlockFilterRequest;
}

export namespace GetBlockFilterRequest {
  export type AsObject = {
    hash: Uint8Array | string,
    height: number,
  }

  export enum HashOrHeightCase {
    HASH_OR_HEIGHT_NOT_SET = 0,
    HASH = 1,
    HEIGHT = 2,
  }
}

export class GetBlockFilterResponse extends jspb.Message {
  getFilter(): Uint8Array | string;
  getFilter_asU8(): Uint8Array;
  getFilter_asB64(): string;
  setFilter(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetBlockFilterResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetBlockFilterResponse): GetBlockFilterResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetBlockFilterResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetBlockFilterResponse;
  static deserializeBinaryFromReader(message: GetBlockFilterResponse, reader: jspb.BinaryReader): GetBlockFilterResponse;
}

export namespace GetBlockFilterResponse {
  export type AsObject = {
    filter: Uint8Array | string,
  }
}

export class GetHeadersRequest extends jspb.Message {
  clearBlockLocatorHashesList(): void;
  getBlockLocatorHashesList(): Array<Uint8Array | string>;
  getBlockLocatorHashesList_asU8(): Array<Uint8Array>;
  getBlockLocatorHashesList_asB64(): Array<string>;
  setBlockLocatorHashesList(value: Array<Uint8Array | string>): void;
  addBlockLocatorHashes(value: Uint8Array | string, index?: number): Uint8Array | string;

  getStopHash(): Uint8Array | string;
  getStopHash_asU8(): Uint8Array;
  getStopHash_asB64(): string;
  setStopHash(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetHeadersRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetHeadersRequest): GetHeadersRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetHeadersRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetHeadersRequest;
  static deserializeBinaryFromReader(message: GetHeadersRequest, reader: jspb.BinaryReader): GetHeadersRequest;
}

export namespace GetHeadersRequest {
  export type AsObject = {
    blockLocatorHashesList: Array<Uint8Array | string>,
    stopHash: Uint8Array | string,
  }
}

export class GetHeadersResponse extends jspb.Message {
  clearHeadersList(): void;
  getHeadersList(): Array<BlockInfo>;
  setHeadersList(value: Array<BlockInfo>): void;
  addHeaders(value?: BlockInfo, index?: number): BlockInfo;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetHeadersResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetHeadersResponse): GetHeadersResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetHeadersResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetHeadersResponse;
  static deserializeBinaryFromReader(message: GetHeadersResponse, reader: jspb.BinaryReader): GetHeadersResponse;
}

export namespace GetHeadersResponse {
  export type AsObject = {
    headersList: Array<BlockInfo.AsObject>,
  }
}

export class GetTransactionRequest extends jspb.Message {
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetTransactionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetTransactionRequest): GetTransactionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetTransactionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetTransactionRequest;
  static deserializeBinaryFromReader(message: GetTransactionRequest, reader: jspb.BinaryReader): GetTransactionRequest;
}

export namespace GetTransactionRequest {
  export type AsObject = {
    hash: Uint8Array | string,
  }
}

export class GetTransactionResponse extends jspb.Message {
  hasTransaction(): boolean;
  clearTransaction(): void;
  getTransaction(): Transaction | undefined;
  setTransaction(value?: Transaction): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetTransactionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetTransactionResponse): GetTransactionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetTransactionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetTransactionResponse;
  static deserializeBinaryFromReader(message: GetTransactionResponse, reader: jspb.BinaryReader): GetTransactionResponse;
}

export namespace GetTransactionResponse {
  export type AsObject = {
    transaction?: Transaction.AsObject,
  }
}

export class GetRawTransactionRequest extends jspb.Message {
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetRawTransactionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetRawTransactionRequest): GetRawTransactionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetRawTransactionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetRawTransactionRequest;
  static deserializeBinaryFromReader(message: GetRawTransactionRequest, reader: jspb.BinaryReader): GetRawTransactionRequest;
}

export namespace GetRawTransactionRequest {
  export type AsObject = {
    hash: Uint8Array | string,
  }
}

export class GetRawTransactionResponse extends jspb.Message {
  getTransaction(): Uint8Array | string;
  getTransaction_asU8(): Uint8Array;
  getTransaction_asB64(): string;
  setTransaction(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetRawTransactionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetRawTransactionResponse): GetRawTransactionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetRawTransactionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetRawTransactionResponse;
  static deserializeBinaryFromReader(message: GetRawTransactionResponse, reader: jspb.BinaryReader): GetRawTransactionResponse;
}

export namespace GetRawTransactionResponse {
  export type AsObject = {
    transaction: Uint8Array | string,
  }
}

export class GetAddressTransactionsRequest extends jspb.Message {
  getAddress(): string;
  setAddress(value: string): void;

  getNbSkip(): number;
  setNbSkip(value: number): void;

  getNbFetch(): number;
  setNbFetch(value: number): void;

  hasHash(): boolean;
  clearHash(): void;
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  hasHeight(): boolean;
  clearHeight(): void;
  getHeight(): number;
  setHeight(value: number): void;

  getStartBlockCase(): GetAddressTransactionsRequest.StartBlockCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetAddressTransactionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetAddressTransactionsRequest): GetAddressTransactionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetAddressTransactionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetAddressTransactionsRequest;
  static deserializeBinaryFromReader(message: GetAddressTransactionsRequest, reader: jspb.BinaryReader): GetAddressTransactionsRequest;
}

export namespace GetAddressTransactionsRequest {
  export type AsObject = {
    address: string,
    nbSkip: number,
    nbFetch: number,
    hash: Uint8Array | string,
    height: number,
  }

  export enum StartBlockCase {
    START_BLOCK_NOT_SET = 0,
    HASH = 4,
    HEIGHT = 5,
  }
}

export class GetAddressTransactionsResponse extends jspb.Message {
  clearConfirmedTransactionsList(): void;
  getConfirmedTransactionsList(): Array<Transaction>;
  setConfirmedTransactionsList(value: Array<Transaction>): void;
  addConfirmedTransactions(value?: Transaction, index?: number): Transaction;

  clearUnconfirmedTransactionsList(): void;
  getUnconfirmedTransactionsList(): Array<MempoolTransaction>;
  setUnconfirmedTransactionsList(value: Array<MempoolTransaction>): void;
  addUnconfirmedTransactions(value?: MempoolTransaction, index?: number): MempoolTransaction;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetAddressTransactionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetAddressTransactionsResponse): GetAddressTransactionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetAddressTransactionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetAddressTransactionsResponse;
  static deserializeBinaryFromReader(message: GetAddressTransactionsResponse, reader: jspb.BinaryReader): GetAddressTransactionsResponse;
}

export namespace GetAddressTransactionsResponse {
  export type AsObject = {
    confirmedTransactionsList: Array<Transaction.AsObject>,
    unconfirmedTransactionsList: Array<MempoolTransaction.AsObject>,
  }
}

export class GetRawAddressTransactionsRequest extends jspb.Message {
  getAddress(): string;
  setAddress(value: string): void;

  getNbSkip(): number;
  setNbSkip(value: number): void;

  getNbFetch(): number;
  setNbFetch(value: number): void;

  hasHash(): boolean;
  clearHash(): void;
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  hasHeight(): boolean;
  clearHeight(): void;
  getHeight(): number;
  setHeight(value: number): void;

  getStartBlockCase(): GetRawAddressTransactionsRequest.StartBlockCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetRawAddressTransactionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetRawAddressTransactionsRequest): GetRawAddressTransactionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetRawAddressTransactionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetRawAddressTransactionsRequest;
  static deserializeBinaryFromReader(message: GetRawAddressTransactionsRequest, reader: jspb.BinaryReader): GetRawAddressTransactionsRequest;
}

export namespace GetRawAddressTransactionsRequest {
  export type AsObject = {
    address: string,
    nbSkip: number,
    nbFetch: number,
    hash: Uint8Array | string,
    height: number,
  }

  export enum StartBlockCase {
    START_BLOCK_NOT_SET = 0,
    HASH = 4,
    HEIGHT = 5,
  }
}

export class GetRawAddressTransactionsResponse extends jspb.Message {
  clearConfirmedTransactionsList(): void;
  getConfirmedTransactionsList(): Array<Uint8Array | string>;
  getConfirmedTransactionsList_asU8(): Array<Uint8Array>;
  getConfirmedTransactionsList_asB64(): Array<string>;
  setConfirmedTransactionsList(value: Array<Uint8Array | string>): void;
  addConfirmedTransactions(value: Uint8Array | string, index?: number): Uint8Array | string;

  clearUnconfirmedTransactionsList(): void;
  getUnconfirmedTransactionsList(): Array<Uint8Array | string>;
  getUnconfirmedTransactionsList_asU8(): Array<Uint8Array>;
  getUnconfirmedTransactionsList_asB64(): Array<string>;
  setUnconfirmedTransactionsList(value: Array<Uint8Array | string>): void;
  addUnconfirmedTransactions(value: Uint8Array | string, index?: number): Uint8Array | string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetRawAddressTransactionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetRawAddressTransactionsResponse): GetRawAddressTransactionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetRawAddressTransactionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetRawAddressTransactionsResponse;
  static deserializeBinaryFromReader(message: GetRawAddressTransactionsResponse, reader: jspb.BinaryReader): GetRawAddressTransactionsResponse;
}

export namespace GetRawAddressTransactionsResponse {
  export type AsObject = {
    confirmedTransactionsList: Array<Uint8Array | string>,
    unconfirmedTransactionsList: Array<Uint8Array | string>,
  }
}

export class GetAddressUnspentOutputsRequest extends jspb.Message {
  getAddress(): string;
  setAddress(value: string): void;

  getIncludeMempool(): boolean;
  setIncludeMempool(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetAddressUnspentOutputsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetAddressUnspentOutputsRequest): GetAddressUnspentOutputsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetAddressUnspentOutputsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetAddressUnspentOutputsRequest;
  static deserializeBinaryFromReader(message: GetAddressUnspentOutputsRequest, reader: jspb.BinaryReader): GetAddressUnspentOutputsRequest;
}

export namespace GetAddressUnspentOutputsRequest {
  export type AsObject = {
    address: string,
    includeMempool: boolean,
  }
}

export class GetAddressUnspentOutputsResponse extends jspb.Message {
  clearOutputsList(): void;
  getOutputsList(): Array<UnspentOutput>;
  setOutputsList(value: Array<UnspentOutput>): void;
  addOutputs(value?: UnspentOutput, index?: number): UnspentOutput;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetAddressUnspentOutputsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetAddressUnspentOutputsResponse): GetAddressUnspentOutputsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetAddressUnspentOutputsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetAddressUnspentOutputsResponse;
  static deserializeBinaryFromReader(message: GetAddressUnspentOutputsResponse, reader: jspb.BinaryReader): GetAddressUnspentOutputsResponse;
}

export namespace GetAddressUnspentOutputsResponse {
  export type AsObject = {
    outputsList: Array<UnspentOutput.AsObject>,
  }
}

export class GetUnspentOutputRequest extends jspb.Message {
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  getIndex(): number;
  setIndex(value: number): void;

  getIncludeMempool(): boolean;
  setIncludeMempool(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetUnspentOutputRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetUnspentOutputRequest): GetUnspentOutputRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetUnspentOutputRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetUnspentOutputRequest;
  static deserializeBinaryFromReader(message: GetUnspentOutputRequest, reader: jspb.BinaryReader): GetUnspentOutputRequest;
}

export namespace GetUnspentOutputRequest {
  export type AsObject = {
    hash: Uint8Array | string,
    index: number,
    includeMempool: boolean,
  }
}

export class GetUnspentOutputResponse extends jspb.Message {
  hasOutpoint(): boolean;
  clearOutpoint(): void;
  getOutpoint(): Transaction.Input.Outpoint | undefined;
  setOutpoint(value?: Transaction.Input.Outpoint): void;

  getPubkeyScript(): Uint8Array | string;
  getPubkeyScript_asU8(): Uint8Array;
  getPubkeyScript_asB64(): string;
  setPubkeyScript(value: Uint8Array | string): void;

  getValue(): number;
  setValue(value: number): void;

  getIsCoinbase(): boolean;
  setIsCoinbase(value: boolean): void;

  getBlockHeight(): number;
  setBlockHeight(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetUnspentOutputResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetUnspentOutputResponse): GetUnspentOutputResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetUnspentOutputResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetUnspentOutputResponse;
  static deserializeBinaryFromReader(message: GetUnspentOutputResponse, reader: jspb.BinaryReader): GetUnspentOutputResponse;
}

export namespace GetUnspentOutputResponse {
  export type AsObject = {
    outpoint?: Transaction.Input.Outpoint.AsObject,
    pubkeyScript: Uint8Array | string,
    value: number,
    isCoinbase: boolean,
    blockHeight: number,
  }
}

export class GetMerkleProofRequest extends jspb.Message {
  getTransactionHash(): Uint8Array | string;
  getTransactionHash_asU8(): Uint8Array;
  getTransactionHash_asB64(): string;
  setTransactionHash(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetMerkleProofRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetMerkleProofRequest): GetMerkleProofRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetMerkleProofRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetMerkleProofRequest;
  static deserializeBinaryFromReader(message: GetMerkleProofRequest, reader: jspb.BinaryReader): GetMerkleProofRequest;
}

export namespace GetMerkleProofRequest {
  export type AsObject = {
    transactionHash: Uint8Array | string,
  }
}

export class GetMerkleProofResponse extends jspb.Message {
  hasBlock(): boolean;
  clearBlock(): void;
  getBlock(): BlockInfo | undefined;
  setBlock(value?: BlockInfo): void;

  clearHashesList(): void;
  getHashesList(): Array<Uint8Array | string>;
  getHashesList_asU8(): Array<Uint8Array>;
  getHashesList_asB64(): Array<string>;
  setHashesList(value: Array<Uint8Array | string>): void;
  addHashes(value: Uint8Array | string, index?: number): Uint8Array | string;

  getFlags(): Uint8Array | string;
  getFlags_asU8(): Uint8Array;
  getFlags_asB64(): string;
  setFlags(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetMerkleProofResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetMerkleProofResponse): GetMerkleProofResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetMerkleProofResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetMerkleProofResponse;
  static deserializeBinaryFromReader(message: GetMerkleProofResponse, reader: jspb.BinaryReader): GetMerkleProofResponse;
}

export namespace GetMerkleProofResponse {
  export type AsObject = {
    block?: BlockInfo.AsObject,
    hashesList: Array<Uint8Array | string>,
    flags: Uint8Array | string,
  }
}

export class SubmitTransactionRequest extends jspb.Message {
  getTransaction(): Uint8Array | string;
  getTransaction_asU8(): Uint8Array;
  getTransaction_asB64(): string;
  setTransaction(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SubmitTransactionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SubmitTransactionRequest): SubmitTransactionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SubmitTransactionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SubmitTransactionRequest;
  static deserializeBinaryFromReader(message: SubmitTransactionRequest, reader: jspb.BinaryReader): SubmitTransactionRequest;
}

export namespace SubmitTransactionRequest {
  export type AsObject = {
    transaction: Uint8Array | string,
  }
}

export class SubmitTransactionResponse extends jspb.Message {
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SubmitTransactionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: SubmitTransactionResponse): SubmitTransactionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SubmitTransactionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SubmitTransactionResponse;
  static deserializeBinaryFromReader(message: SubmitTransactionResponse, reader: jspb.BinaryReader): SubmitTransactionResponse;
}

export namespace SubmitTransactionResponse {
  export type AsObject = {
    hash: Uint8Array | string,
  }
}

export class SubscribeTransactionsRequest extends jspb.Message {
  hasSubscribe(): boolean;
  clearSubscribe(): void;
  getSubscribe(): TransactionFilter | undefined;
  setSubscribe(value?: TransactionFilter): void;

  hasUnsubscribe(): boolean;
  clearUnsubscribe(): void;
  getUnsubscribe(): TransactionFilter | undefined;
  setUnsubscribe(value?: TransactionFilter): void;

  getIncludeMempool(): boolean;
  setIncludeMempool(value: boolean): void;

  getIncludeInBlock(): boolean;
  setIncludeInBlock(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SubscribeTransactionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SubscribeTransactionsRequest): SubscribeTransactionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SubscribeTransactionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SubscribeTransactionsRequest;
  static deserializeBinaryFromReader(message: SubscribeTransactionsRequest, reader: jspb.BinaryReader): SubscribeTransactionsRequest;
}

export namespace SubscribeTransactionsRequest {
  export type AsObject = {
    subscribe?: TransactionFilter.AsObject,
    unsubscribe?: TransactionFilter.AsObject,
    includeMempool: boolean,
    includeInBlock: boolean,
  }
}

export class SubscribeBlocksRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SubscribeBlocksRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SubscribeBlocksRequest): SubscribeBlocksRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SubscribeBlocksRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SubscribeBlocksRequest;
  static deserializeBinaryFromReader(message: SubscribeBlocksRequest, reader: jspb.BinaryReader): SubscribeBlocksRequest;
}

export namespace SubscribeBlocksRequest {
  export type AsObject = {
  }
}

export class BlockNotification extends jspb.Message {
  getType(): BlockNotification.TypeMap[keyof BlockNotification.TypeMap];
  setType(value: BlockNotification.TypeMap[keyof BlockNotification.TypeMap]): void;

  hasBlock(): boolean;
  clearBlock(): void;
  getBlock(): BlockInfo | undefined;
  setBlock(value?: BlockInfo): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BlockNotification.AsObject;
  static toObject(includeInstance: boolean, msg: BlockNotification): BlockNotification.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BlockNotification, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BlockNotification;
  static deserializeBinaryFromReader(message: BlockNotification, reader: jspb.BinaryReader): BlockNotification;
}

export namespace BlockNotification {
  export type AsObject = {
    type: BlockNotification.TypeMap[keyof BlockNotification.TypeMap],
    block?: BlockInfo.AsObject,
  }

  export interface TypeMap {
    CONNECTED: 0;
    DISCONNECTED: 1;
  }

  export const Type: TypeMap;
}

export class TransactionNotification extends jspb.Message {
  getType(): TransactionNotification.TypeMap[keyof TransactionNotification.TypeMap];
  setType(value: TransactionNotification.TypeMap[keyof TransactionNotification.TypeMap]): void;

  hasConfirmedTransaction(): boolean;
  clearConfirmedTransaction(): void;
  getConfirmedTransaction(): Transaction | undefined;
  setConfirmedTransaction(value?: Transaction): void;

  hasUnconfirmedTransaction(): boolean;
  clearUnconfirmedTransaction(): void;
  getUnconfirmedTransaction(): MempoolTransaction | undefined;
  setUnconfirmedTransaction(value?: MempoolTransaction): void;

  getTransactionCase(): TransactionNotification.TransactionCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): TransactionNotification.AsObject;
  static toObject(includeInstance: boolean, msg: TransactionNotification): TransactionNotification.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: TransactionNotification, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): TransactionNotification;
  static deserializeBinaryFromReader(message: TransactionNotification, reader: jspb.BinaryReader): TransactionNotification;
}

export namespace TransactionNotification {
  export type AsObject = {
    type: TransactionNotification.TypeMap[keyof TransactionNotification.TypeMap],
    confirmedTransaction?: Transaction.AsObject,
    unconfirmedTransaction?: MempoolTransaction.AsObject,
  }

  export interface TypeMap {
    UNCONFIRMED: 0;
    CONFIRMED: 1;
  }

  export const Type: TypeMap;

  export enum TransactionCase {
    TRANSACTION_NOT_SET = 0,
    CONFIRMED_TRANSACTION = 2,
    UNCONFIRMED_TRANSACTION = 3,
  }
}

export class BlockInfo extends jspb.Message {
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  getHeight(): number;
  setHeight(value: number): void;

  getVersion(): number;
  setVersion(value: number): void;

  getPreviousBlock(): Uint8Array | string;
  getPreviousBlock_asU8(): Uint8Array;
  getPreviousBlock_asB64(): string;
  setPreviousBlock(value: Uint8Array | string): void;

  getMerkleRoot(): Uint8Array | string;
  getMerkleRoot_asU8(): Uint8Array;
  getMerkleRoot_asB64(): string;
  setMerkleRoot(value: Uint8Array | string): void;

  getTimestamp(): number;
  setTimestamp(value: number): void;

  getBits(): number;
  setBits(value: number): void;

  getNonce(): number;
  setNonce(value: number): void;

  getConfirmations(): number;
  setConfirmations(value: number): void;

  getDifficulty(): number;
  setDifficulty(value: number): void;

  getNextBlockHash(): Uint8Array | string;
  getNextBlockHash_asU8(): Uint8Array;
  getNextBlockHash_asB64(): string;
  setNextBlockHash(value: Uint8Array | string): void;

  getSize(): number;
  setSize(value: number): void;

  getMedianTime(): number;
  setMedianTime(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BlockInfo.AsObject;
  static toObject(includeInstance: boolean, msg: BlockInfo): BlockInfo.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BlockInfo, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BlockInfo;
  static deserializeBinaryFromReader(message: BlockInfo, reader: jspb.BinaryReader): BlockInfo;
}

export namespace BlockInfo {
  export type AsObject = {
    hash: Uint8Array | string,
    height: number,
    version: number,
    previousBlock: Uint8Array | string,
    merkleRoot: Uint8Array | string,
    timestamp: number,
    bits: number,
    nonce: number,
    confirmations: number,
    difficulty: number,
    nextBlockHash: Uint8Array | string,
    size: number,
    medianTime: number,
  }
}

export class Block extends jspb.Message {
  hasInfo(): boolean;
  clearInfo(): void;
  getInfo(): BlockInfo | undefined;
  setInfo(value?: BlockInfo): void;

  clearTransactionDataList(): void;
  getTransactionDataList(): Array<Block.TransactionData>;
  setTransactionDataList(value: Array<Block.TransactionData>): void;
  addTransactionData(value?: Block.TransactionData, index?: number): Block.TransactionData;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Block.AsObject;
  static toObject(includeInstance: boolean, msg: Block): Block.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Block, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Block;
  static deserializeBinaryFromReader(message: Block, reader: jspb.BinaryReader): Block;
}

export namespace Block {
  export type AsObject = {
    info?: BlockInfo.AsObject,
    transactionDataList: Array<Block.TransactionData.AsObject>,
  }

  export class TransactionData extends jspb.Message {
    hasTransactionHash(): boolean;
    clearTransactionHash(): void;
    getTransactionHash(): Uint8Array | string;
    getTransactionHash_asU8(): Uint8Array;
    getTransactionHash_asB64(): string;
    setTransactionHash(value: Uint8Array | string): void;

    hasTransaction(): boolean;
    clearTransaction(): void;
    getTransaction(): Transaction | undefined;
    setTransaction(value?: Transaction): void;

    getTxidsOrTxsCase(): TransactionData.TxidsOrTxsCase;
    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TransactionData.AsObject;
    static toObject(includeInstance: boolean, msg: TransactionData): TransactionData.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TransactionData, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TransactionData;
    static deserializeBinaryFromReader(message: TransactionData, reader: jspb.BinaryReader): TransactionData;
  }

  export namespace TransactionData {
    export type AsObject = {
      transactionHash: Uint8Array | string,
      transaction?: Transaction.AsObject,
    }

    export enum TxidsOrTxsCase {
      TXIDS_OR_TXS_NOT_SET = 0,
      TRANSACTION_HASH = 1,
      TRANSACTION = 2,
    }
  }
}

export class Transaction extends jspb.Message {
  getHash(): Uint8Array | string;
  getHash_asU8(): Uint8Array;
  getHash_asB64(): string;
  setHash(value: Uint8Array | string): void;

  getVersion(): number;
  setVersion(value: number): void;

  clearInputsList(): void;
  getInputsList(): Array<Transaction.Input>;
  setInputsList(value: Array<Transaction.Input>): void;
  addInputs(value?: Transaction.Input, index?: number): Transaction.Input;

  clearOutputsList(): void;
  getOutputsList(): Array<Transaction.Output>;
  setOutputsList(value: Array<Transaction.Output>): void;
  addOutputs(value?: Transaction.Output, index?: number): Transaction.Output;

  getLockTime(): number;
  setLockTime(value: number): void;

  getSize(): number;
  setSize(value: number): void;

  getTimestamp(): number;
  setTimestamp(value: number): void;

  getConfirmations(): number;
  setConfirmations(value: number): void;

  getBlockHeight(): number;
  setBlockHeight(value: number): void;

  getBlockHash(): Uint8Array | string;
  getBlockHash_asU8(): Uint8Array;
  getBlockHash_asB64(): string;
  setBlockHash(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Transaction.AsObject;
  static toObject(includeInstance: boolean, msg: Transaction): Transaction.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Transaction, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Transaction;
  static deserializeBinaryFromReader(message: Transaction, reader: jspb.BinaryReader): Transaction;
}

export namespace Transaction {
  export type AsObject = {
    hash: Uint8Array | string,
    version: number,
    inputsList: Array<Transaction.Input.AsObject>,
    outputsList: Array<Transaction.Output.AsObject>,
    lockTime: number,
    size: number,
    timestamp: number,
    confirmations: number,
    blockHeight: number,
    blockHash: Uint8Array | string,
  }

  export class Input extends jspb.Message {
    getIndex(): number;
    setIndex(value: number): void;

    hasOutpoint(): boolean;
    clearOutpoint(): void;
    getOutpoint(): Transaction.Input.Outpoint | undefined;
    setOutpoint(value?: Transaction.Input.Outpoint): void;

    getSignatureScript(): Uint8Array | string;
    getSignatureScript_asU8(): Uint8Array;
    getSignatureScript_asB64(): string;
    setSignatureScript(value: Uint8Array | string): void;

    getSequence(): number;
    setSequence(value: number): void;

    getValue(): number;
    setValue(value: number): void;

    getPreviousScript(): Uint8Array | string;
    getPreviousScript_asU8(): Uint8Array;
    getPreviousScript_asB64(): string;
    setPreviousScript(value: Uint8Array | string): void;

    getAddress(): string;
    setAddress(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Input.AsObject;
    static toObject(includeInstance: boolean, msg: Input): Input.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Input, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Input;
    static deserializeBinaryFromReader(message: Input, reader: jspb.BinaryReader): Input;
  }

  export namespace Input {
    export type AsObject = {
      index: number,
      outpoint?: Transaction.Input.Outpoint.AsObject,
      signatureScript: Uint8Array | string,
      sequence: number,
      value: number,
      previousScript: Uint8Array | string,
      address: string,
    }

    export class Outpoint extends jspb.Message {
      getHash(): Uint8Array | string;
      getHash_asU8(): Uint8Array;
      getHash_asB64(): string;
      setHash(value: Uint8Array | string): void;

      getIndex(): number;
      setIndex(value: number): void;

      serializeBinary(): Uint8Array;
      toObject(includeInstance?: boolean): Outpoint.AsObject;
      static toObject(includeInstance: boolean, msg: Outpoint): Outpoint.AsObject;
      static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
      static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
      static serializeBinaryToWriter(message: Outpoint, writer: jspb.BinaryWriter): void;
      static deserializeBinary(bytes: Uint8Array): Outpoint;
      static deserializeBinaryFromReader(message: Outpoint, reader: jspb.BinaryReader): Outpoint;
    }

    export namespace Outpoint {
      export type AsObject = {
        hash: Uint8Array | string,
        index: number,
      }
    }
  }

  export class Output extends jspb.Message {
    getIndex(): number;
    setIndex(value: number): void;

    getValue(): number;
    setValue(value: number): void;

    getPubkeyScript(): Uint8Array | string;
    getPubkeyScript_asU8(): Uint8Array;
    getPubkeyScript_asB64(): string;
    setPubkeyScript(value: Uint8Array | string): void;

    getAddress(): string;
    setAddress(value: string): void;

    getScriptClass(): string;
    setScriptClass(value: string): void;

    getDisassembledScript(): string;
    setDisassembledScript(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Output.AsObject;
    static toObject(includeInstance: boolean, msg: Output): Output.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Output, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Output;
    static deserializeBinaryFromReader(message: Output, reader: jspb.BinaryReader): Output;
  }

  export namespace Output {
    export type AsObject = {
      index: number,
      value: number,
      pubkeyScript: Uint8Array | string,
      address: string,
      scriptClass: string,
      disassembledScript: string,
    }
  }
}

export class MempoolTransaction extends jspb.Message {
  hasTransaction(): boolean;
  clearTransaction(): void;
  getTransaction(): Transaction | undefined;
  setTransaction(value?: Transaction): void;

  getAddedTime(): number;
  setAddedTime(value: number): void;

  getAddedHeight(): number;
  setAddedHeight(value: number): void;

  getFee(): number;
  setFee(value: number): void;

  getFeePerKb(): number;
  setFeePerKb(value: number): void;

  getStartingPriority(): number;
  setStartingPriority(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MempoolTransaction.AsObject;
  static toObject(includeInstance: boolean, msg: MempoolTransaction): MempoolTransaction.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MempoolTransaction, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MempoolTransaction;
  static deserializeBinaryFromReader(message: MempoolTransaction, reader: jspb.BinaryReader): MempoolTransaction;
}

export namespace MempoolTransaction {
  export type AsObject = {
    transaction?: Transaction.AsObject,
    addedTime: number,
    addedHeight: number,
    fee: number,
    feePerKb: number,
    startingPriority: number,
  }
}

export class UnspentOutput extends jspb.Message {
  hasOutpoint(): boolean;
  clearOutpoint(): void;
  getOutpoint(): Transaction.Input.Outpoint | undefined;
  setOutpoint(value?: Transaction.Input.Outpoint): void;

  getPubkeyScript(): Uint8Array | string;
  getPubkeyScript_asU8(): Uint8Array;
  getPubkeyScript_asB64(): string;
  setPubkeyScript(value: Uint8Array | string): void;

  getValue(): number;
  setValue(value: number): void;

  getIsCoinbase(): boolean;
  setIsCoinbase(value: boolean): void;

  getBlockHeight(): number;
  setBlockHeight(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UnspentOutput.AsObject;
  static toObject(includeInstance: boolean, msg: UnspentOutput): UnspentOutput.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UnspentOutput, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UnspentOutput;
  static deserializeBinaryFromReader(message: UnspentOutput, reader: jspb.BinaryReader): UnspentOutput;
}

export namespace UnspentOutput {
  export type AsObject = {
    outpoint?: Transaction.Input.Outpoint.AsObject,
    pubkeyScript: Uint8Array | string,
    value: number,
    isCoinbase: boolean,
    blockHeight: number,
  }
}

export class TransactionFilter extends jspb.Message {
  clearAddressesList(): void;
  getAddressesList(): Array<string>;
  setAddressesList(value: Array<string>): void;
  addAddresses(value: string, index?: number): string;

  clearOutpointsList(): void;
  getOutpointsList(): Array<Transaction.Input.Outpoint>;
  setOutpointsList(value: Array<Transaction.Input.Outpoint>): void;
  addOutpoints(value?: Transaction.Input.Outpoint, index?: number): Transaction.Input.Outpoint;

  clearDataElementsList(): void;
  getDataElementsList(): Array<Uint8Array | string>;
  getDataElementsList_asU8(): Array<Uint8Array>;
  getDataElementsList_asB64(): Array<string>;
  setDataElementsList(value: Array<Uint8Array | string>): void;
  addDataElements(value: Uint8Array | string, index?: number): Uint8Array | string;

  getAllTransactions(): boolean;
  setAllTransactions(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): TransactionFilter.AsObject;
  static toObject(includeInstance: boolean, msg: TransactionFilter): TransactionFilter.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: TransactionFilter, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): TransactionFilter;
  static deserializeBinaryFromReader(message: TransactionFilter, reader: jspb.BinaryReader): TransactionFilter;
}

export namespace TransactionFilter {
  export type AsObject = {
    addressesList: Array<string>,
    outpointsList: Array<Transaction.Input.Outpoint.AsObject>,
    dataElementsList: Array<Uint8Array | string>,
    allTransactions: boolean,
  }
}

