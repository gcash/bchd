import fs from "fs";
import * as grpc from "@grpc/grpc-js";
import * as pb from "../../pb-js/bchrpc_pb";
import { bchrpc } from "../../pb-js/bchrpc_pb_service";

// Re-export the message classes so tests import them from one place.
export * from "../../pb-js/bchrpc_pb";

type Service = typeof bchrpc;
type MethodName = Exclude<keyof Service, "prototype" | "serviceName">;
type RequestOf<K extends MethodName> = InstanceType<Service[K]["requestType"]>;
type ResponseOf<K extends MethodName> = InstanceType<Service[K]["responseType"]>;
type UnaryMethod = {
  [K in MethodName]: Service[K] extends { requestStream: false; responseStream: false } ? K : never
}[MethodName];
type ServerStreamMethod = {
  [K in MethodName]: Service[K] extends { requestStream: false; responseStream: true } ? K : never
}[MethodName];

interface Message {
  serializeBinary(): Uint8Array;
}

interface MethodDescriptor {
  methodName: string;
  requestStream: boolean;
  responseStream: boolean;
  requestType: { deserializeBinary(bytes: Uint8Array): Message };
  responseType: { deserializeBinary(bytes: Uint8Array): Message };
}

// Build a grpc-js service definition from the method table that protoc
// generated for bchrpc.proto along with the message classes, so the client
// has every RPC in the service and its serialization matches the classes.
const definition: Record<string, grpc.MethodDefinition<Message, Message>> = {};
for (const [name, value] of Object.entries(bchrpc)) {
  if (typeof value !== "object" || value === null || !("methodName" in value)) {
    continue;
  }
  const method = value as MethodDescriptor;
  definition[name] = {
    path: `/${bchrpc.serviceName}/${method.methodName}`,
    requestStream: method.requestStream,
    responseStream: method.responseStream,
    requestSerialize: (msg: Message) => Buffer.from(msg.serializeBinary()),
    requestDeserialize: (buf: Buffer) => method.requestType.deserializeBinary(buf),
    responseSerialize: (msg: Message) => Buffer.from(msg.serializeBinary()),
    responseDeserialize: (buf: Buffer) => method.responseType.deserializeBinary(buf),
  };
}
const BchrpcClient = grpc.makeGenericClientConstructor(definition, bchrpc.serviceName);

// hexToBytes decodes a hex hash, reversing it if it is in the order a hash
// is displayed in.
const hexToBytes = (hex: string, reversed: boolean): Uint8Array => {
  const bytes = Buffer.from(hex, "hex");
  return reversed ? bytes.reverse() : bytes;
};

export interface GrpcClientOptions {
  url: string;
  rootCertPath: string;
  // serverName is the name to expect in the server's certificate when it
  // differs from the host in url.
  serverName?: string;
}

// GrpcClient calls bchd's bchrpc service.  rpc has a method for every RPC,
// named as in bchrpc.proto, and unary and serverStream type the common
// cases.  The other methods keep the defaults of grpc-bchrpc-node's
// GrpcClient, which the tests were written against.
export class GrpcClient {
  readonly rpc: InstanceType<typeof BchrpcClient>;
  private slpIndex = false;

  constructor({ url, rootCertPath, serverName }: GrpcClientOptions) {
    const credentials = grpc.credentials.createSsl(fs.readFileSync(rootCertPath));
    const options: grpc.ChannelOptions = { "grpc.max_receive_message_length": -1 };
    if (serverName) {
      options["grpc.ssl_target_name_override"] = serverName;
      options["grpc.default_authority"] = serverName;
    }
    this.rpc = new BchrpcClient(url, credentials, options);
  }

  unary<K extends UnaryMethod>(method: K, request: RequestOf<K>): Promise<ResponseOf<K>> {
    return new Promise((resolve, reject) => {
      this.rpc[method](request, (err: grpc.ServiceError | null, res: ResponseOf<K>) => {
        if (err) {
          reject(err);
        } else {
          resolve(res);
        }
      });
    });
  }

  serverStream<K extends ServerStreamMethod>(method: K, request: RequestOf<K>): grpc.ClientReadableStream<ResponseOf<K>> {
    return this.rpc[method](request) as grpc.ClientReadableStream<ResponseOf<K>>;
  }

  getBlockchainInfo(): Promise<pb.GetBlockchainInfoResponse> {
    return this.unary("GetBlockchainInfo", new pb.GetBlockchainInfoRequest());
  }

  getAddressUtxos({ address, includeMempool = false, includeTokenMetadata = true }: {
    address: string; includeMempool?: boolean; includeTokenMetadata?: boolean;
  }): Promise<pb.GetAddressUnspentOutputsResponse> {
    const req = new pb.GetAddressUnspentOutputsRequest();
    req.setAddress(address);
    req.setIncludeMempool(includeMempool);
    req.setIncludeTokenMetadata(includeTokenMetadata);
    return this.unary("GetAddressUnspentOutputs", req);
  }

  getTransaction({ hash, reversedHashOrder = false, includeTokenMetadata = true }: {
    hash: string; reversedHashOrder?: boolean; includeTokenMetadata?: boolean;
  }): Promise<pb.GetTransactionResponse> {
    const req = new pb.GetTransactionRequest();
    req.setHash(hexToBytes(hash, reversedHashOrder));
    req.setIncludeTokenMetadata(includeTokenMetadata);
    return this.unary("GetTransaction", req);
  }

  // submitTransaction refuses to submit unless bchd has the SLP index, which
  // the SLP validity checks need, or skipSlpValidityChecks is set.
  async submitTransaction({ txnHex, skipSlpValidityChecks = false }: {
    txnHex: string; skipSlpValidityChecks?: boolean;
  }): Promise<pb.SubmitTransactionResponse> {
    const req = new pb.SubmitTransactionRequest();
    req.setTransaction(Buffer.from(txnHex, "hex"));
    if (skipSlpValidityChecks) {
      req.setSkipSlpValidityCheck(true);
    } else if (!this.slpIndex) {
      this.slpIndex = (await this.getBlockchainInfo()).getSlpIndex();
      if (!this.slpIndex) {
        throw new Error("bchd does not have the SLP index enabled");
      }
    }
    return this.unary("SubmitTransaction", req);
  }
}
