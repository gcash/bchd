// package: pb
// file: bchrpc.proto

import * as bchrpc_pb from "./bchrpc_pb";
import {grpc} from "@improbable-eng/grpc-web";

type bchrpcGetMempoolInfo = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetMempoolInfoRequest;
  readonly responseType: typeof bchrpc_pb.GetMempoolInfoResponse;
};

type bchrpcGetMempool = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetMempoolRequest;
  readonly responseType: typeof bchrpc_pb.GetMempoolResponse;
};

type bchrpcGetBlockchainInfo = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetBlockchainInfoRequest;
  readonly responseType: typeof bchrpc_pb.GetBlockchainInfoResponse;
};

type bchrpcGetBlockInfo = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetBlockInfoRequest;
  readonly responseType: typeof bchrpc_pb.GetBlockInfoResponse;
};

type bchrpcGetBlock = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetBlockRequest;
  readonly responseType: typeof bchrpc_pb.GetBlockResponse;
};

type bchrpcGetRawBlock = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetRawBlockRequest;
  readonly responseType: typeof bchrpc_pb.GetRawBlockResponse;
};

type bchrpcGetBlockFilter = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetBlockFilterRequest;
  readonly responseType: typeof bchrpc_pb.GetBlockFilterResponse;
};

type bchrpcGetHeaders = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetHeadersRequest;
  readonly responseType: typeof bchrpc_pb.GetHeadersResponse;
};

type bchrpcGetTransaction = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetTransactionRequest;
  readonly responseType: typeof bchrpc_pb.GetTransactionResponse;
};

type bchrpcGetRawTransaction = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetRawTransactionRequest;
  readonly responseType: typeof bchrpc_pb.GetRawTransactionResponse;
};

type bchrpcGetAddressTransactions = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetAddressTransactionsRequest;
  readonly responseType: typeof bchrpc_pb.GetAddressTransactionsResponse;
};

type bchrpcGetRawAddressTransactions = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetRawAddressTransactionsRequest;
  readonly responseType: typeof bchrpc_pb.GetRawAddressTransactionsResponse;
};

type bchrpcGetAddressUnspentOutputs = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetAddressUnspentOutputsRequest;
  readonly responseType: typeof bchrpc_pb.GetAddressUnspentOutputsResponse;
};

type bchrpcGetUnspentOutput = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetUnspentOutputRequest;
  readonly responseType: typeof bchrpc_pb.GetUnspentOutputResponse;
};

type bchrpcGetMerkleProof = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetMerkleProofRequest;
  readonly responseType: typeof bchrpc_pb.GetMerkleProofResponse;
};

type bchrpcGetSlpTokenMetadata = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetSlpTokenMetadataRequest;
  readonly responseType: typeof bchrpc_pb.GetSlpTokenMetadataResponse;
};

type bchrpcGetSlpParsedScript = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetSlpParsedScriptRequest;
  readonly responseType: typeof bchrpc_pb.GetSlpParsedScriptResponse;
};

type bchrpcGetSlpTrustedValidation = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetSlpTrustedValidationRequest;
  readonly responseType: typeof bchrpc_pb.GetSlpTrustedValidationResponse;
};

type bchrpcGetSlpGraphSearch = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.GetSlpGraphSearchRequest;
  readonly responseType: typeof bchrpc_pb.GetSlpGraphSearchResponse;
};

type bchrpcCheckSlpTransaction = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.CheckSlpTransactionRequest;
  readonly responseType: typeof bchrpc_pb.CheckSlpTransactionResponse;
};

type bchrpcSubmitTransaction = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof bchrpc_pb.SubmitTransactionRequest;
  readonly responseType: typeof bchrpc_pb.SubmitTransactionResponse;
};

type bchrpcSubscribeTransactions = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof bchrpc_pb.SubscribeTransactionsRequest;
  readonly responseType: typeof bchrpc_pb.TransactionNotification;
};

type bchrpcSubscribeTransactionStream = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: true;
  readonly responseStream: true;
  readonly requestType: typeof bchrpc_pb.SubscribeTransactionsRequest;
  readonly responseType: typeof bchrpc_pb.TransactionNotification;
};

type bchrpcSubscribeBlocks = {
  readonly methodName: string;
  readonly service: typeof bchrpc;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof bchrpc_pb.SubscribeBlocksRequest;
  readonly responseType: typeof bchrpc_pb.BlockNotification;
};

export class bchrpc {
  static readonly serviceName: string;
  static readonly GetMempoolInfo: bchrpcGetMempoolInfo;
  static readonly GetMempool: bchrpcGetMempool;
  static readonly GetBlockchainInfo: bchrpcGetBlockchainInfo;
  static readonly GetBlockInfo: bchrpcGetBlockInfo;
  static readonly GetBlock: bchrpcGetBlock;
  static readonly GetRawBlock: bchrpcGetRawBlock;
  static readonly GetBlockFilter: bchrpcGetBlockFilter;
  static readonly GetHeaders: bchrpcGetHeaders;
  static readonly GetTransaction: bchrpcGetTransaction;
  static readonly GetRawTransaction: bchrpcGetRawTransaction;
  static readonly GetAddressTransactions: bchrpcGetAddressTransactions;
  static readonly GetRawAddressTransactions: bchrpcGetRawAddressTransactions;
  static readonly GetAddressUnspentOutputs: bchrpcGetAddressUnspentOutputs;
  static readonly GetUnspentOutput: bchrpcGetUnspentOutput;
  static readonly GetMerkleProof: bchrpcGetMerkleProof;
  static readonly GetSlpTokenMetadata: bchrpcGetSlpTokenMetadata;
  static readonly GetSlpParsedScript: bchrpcGetSlpParsedScript;
  static readonly GetSlpTrustedValidation: bchrpcGetSlpTrustedValidation;
  static readonly GetSlpGraphSearch: bchrpcGetSlpGraphSearch;
  static readonly CheckSlpTransaction: bchrpcCheckSlpTransaction;
  static readonly SubmitTransaction: bchrpcSubmitTransaction;
  static readonly SubscribeTransactions: bchrpcSubscribeTransactions;
  static readonly SubscribeTransactionStream: bchrpcSubscribeTransactionStream;
  static readonly SubscribeBlocks: bchrpcSubscribeBlocks;
}

export type ServiceError = { message: string, code: number; metadata: grpc.Metadata }
export type Status = { details: string, code: number; metadata: grpc.Metadata }

interface UnaryResponse {
  cancel(): void;
}
interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: (status?: Status) => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}
interface RequestStream<T> {
  write(message: T): RequestStream<T>;
  end(): void;
  cancel(): void;
  on(type: 'end', handler: (status?: Status) => void): RequestStream<T>;
  on(type: 'status', handler: (status: Status) => void): RequestStream<T>;
}
interface BidirectionalStream<ReqT, ResT> {
  write(message: ReqT): BidirectionalStream<ReqT, ResT>;
  end(): void;
  cancel(): void;
  on(type: 'data', handler: (message: ResT) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'end', handler: (status?: Status) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'status', handler: (status: Status) => void): BidirectionalStream<ReqT, ResT>;
}

export class bchrpcClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  getMempoolInfo(
    requestMessage: bchrpc_pb.GetMempoolInfoRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetMempoolInfoResponse|null) => void
  ): UnaryResponse;
  getMempoolInfo(
    requestMessage: bchrpc_pb.GetMempoolInfoRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetMempoolInfoResponse|null) => void
  ): UnaryResponse;
  getMempool(
    requestMessage: bchrpc_pb.GetMempoolRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetMempoolResponse|null) => void
  ): UnaryResponse;
  getMempool(
    requestMessage: bchrpc_pb.GetMempoolRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetMempoolResponse|null) => void
  ): UnaryResponse;
  getBlockchainInfo(
    requestMessage: bchrpc_pb.GetBlockchainInfoRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetBlockchainInfoResponse|null) => void
  ): UnaryResponse;
  getBlockchainInfo(
    requestMessage: bchrpc_pb.GetBlockchainInfoRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetBlockchainInfoResponse|null) => void
  ): UnaryResponse;
  getBlockInfo(
    requestMessage: bchrpc_pb.GetBlockInfoRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetBlockInfoResponse|null) => void
  ): UnaryResponse;
  getBlockInfo(
    requestMessage: bchrpc_pb.GetBlockInfoRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetBlockInfoResponse|null) => void
  ): UnaryResponse;
  getBlock(
    requestMessage: bchrpc_pb.GetBlockRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetBlockResponse|null) => void
  ): UnaryResponse;
  getBlock(
    requestMessage: bchrpc_pb.GetBlockRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetBlockResponse|null) => void
  ): UnaryResponse;
  getRawBlock(
    requestMessage: bchrpc_pb.GetRawBlockRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetRawBlockResponse|null) => void
  ): UnaryResponse;
  getRawBlock(
    requestMessage: bchrpc_pb.GetRawBlockRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetRawBlockResponse|null) => void
  ): UnaryResponse;
  getBlockFilter(
    requestMessage: bchrpc_pb.GetBlockFilterRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetBlockFilterResponse|null) => void
  ): UnaryResponse;
  getBlockFilter(
    requestMessage: bchrpc_pb.GetBlockFilterRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetBlockFilterResponse|null) => void
  ): UnaryResponse;
  getHeaders(
    requestMessage: bchrpc_pb.GetHeadersRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetHeadersResponse|null) => void
  ): UnaryResponse;
  getHeaders(
    requestMessage: bchrpc_pb.GetHeadersRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetHeadersResponse|null) => void
  ): UnaryResponse;
  getTransaction(
    requestMessage: bchrpc_pb.GetTransactionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetTransactionResponse|null) => void
  ): UnaryResponse;
  getTransaction(
    requestMessage: bchrpc_pb.GetTransactionRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetTransactionResponse|null) => void
  ): UnaryResponse;
  getRawTransaction(
    requestMessage: bchrpc_pb.GetRawTransactionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetRawTransactionResponse|null) => void
  ): UnaryResponse;
  getRawTransaction(
    requestMessage: bchrpc_pb.GetRawTransactionRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetRawTransactionResponse|null) => void
  ): UnaryResponse;
  getAddressTransactions(
    requestMessage: bchrpc_pb.GetAddressTransactionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetAddressTransactionsResponse|null) => void
  ): UnaryResponse;
  getAddressTransactions(
    requestMessage: bchrpc_pb.GetAddressTransactionsRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetAddressTransactionsResponse|null) => void
  ): UnaryResponse;
  getRawAddressTransactions(
    requestMessage: bchrpc_pb.GetRawAddressTransactionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetRawAddressTransactionsResponse|null) => void
  ): UnaryResponse;
  getRawAddressTransactions(
    requestMessage: bchrpc_pb.GetRawAddressTransactionsRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetRawAddressTransactionsResponse|null) => void
  ): UnaryResponse;
  getAddressUnspentOutputs(
    requestMessage: bchrpc_pb.GetAddressUnspentOutputsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetAddressUnspentOutputsResponse|null) => void
  ): UnaryResponse;
  getAddressUnspentOutputs(
    requestMessage: bchrpc_pb.GetAddressUnspentOutputsRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetAddressUnspentOutputsResponse|null) => void
  ): UnaryResponse;
  getUnspentOutput(
    requestMessage: bchrpc_pb.GetUnspentOutputRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetUnspentOutputResponse|null) => void
  ): UnaryResponse;
  getUnspentOutput(
    requestMessage: bchrpc_pb.GetUnspentOutputRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetUnspentOutputResponse|null) => void
  ): UnaryResponse;
  getMerkleProof(
    requestMessage: bchrpc_pb.GetMerkleProofRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetMerkleProofResponse|null) => void
  ): UnaryResponse;
  getMerkleProof(
    requestMessage: bchrpc_pb.GetMerkleProofRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetMerkleProofResponse|null) => void
  ): UnaryResponse;
  getSlpTokenMetadata(
    requestMessage: bchrpc_pb.GetSlpTokenMetadataRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetSlpTokenMetadataResponse|null) => void
  ): UnaryResponse;
  getSlpTokenMetadata(
    requestMessage: bchrpc_pb.GetSlpTokenMetadataRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetSlpTokenMetadataResponse|null) => void
  ): UnaryResponse;
  getSlpParsedScript(
    requestMessage: bchrpc_pb.GetSlpParsedScriptRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetSlpParsedScriptResponse|null) => void
  ): UnaryResponse;
  getSlpParsedScript(
    requestMessage: bchrpc_pb.GetSlpParsedScriptRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetSlpParsedScriptResponse|null) => void
  ): UnaryResponse;
  getSlpTrustedValidation(
    requestMessage: bchrpc_pb.GetSlpTrustedValidationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetSlpTrustedValidationResponse|null) => void
  ): UnaryResponse;
  getSlpTrustedValidation(
    requestMessage: bchrpc_pb.GetSlpTrustedValidationRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetSlpTrustedValidationResponse|null) => void
  ): UnaryResponse;
  getSlpGraphSearch(
    requestMessage: bchrpc_pb.GetSlpGraphSearchRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetSlpGraphSearchResponse|null) => void
  ): UnaryResponse;
  getSlpGraphSearch(
    requestMessage: bchrpc_pb.GetSlpGraphSearchRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.GetSlpGraphSearchResponse|null) => void
  ): UnaryResponse;
  checkSlpTransaction(
    requestMessage: bchrpc_pb.CheckSlpTransactionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.CheckSlpTransactionResponse|null) => void
  ): UnaryResponse;
  checkSlpTransaction(
    requestMessage: bchrpc_pb.CheckSlpTransactionRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.CheckSlpTransactionResponse|null) => void
  ): UnaryResponse;
  submitTransaction(
    requestMessage: bchrpc_pb.SubmitTransactionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.SubmitTransactionResponse|null) => void
  ): UnaryResponse;
  submitTransaction(
    requestMessage: bchrpc_pb.SubmitTransactionRequest,
    callback: (error: ServiceError|null, responseMessage: bchrpc_pb.SubmitTransactionResponse|null) => void
  ): UnaryResponse;
  subscribeTransactions(requestMessage: bchrpc_pb.SubscribeTransactionsRequest, metadata?: grpc.Metadata): ResponseStream<bchrpc_pb.TransactionNotification>;
  subscribeTransactionStream(metadata?: grpc.Metadata): BidirectionalStream<bchrpc_pb.SubscribeTransactionsRequest, bchrpc_pb.TransactionNotification>;
  subscribeBlocks(requestMessage: bchrpc_pb.SubscribeBlocksRequest, metadata?: grpc.Metadata): ResponseStream<bchrpc_pb.BlockNotification>;
}

