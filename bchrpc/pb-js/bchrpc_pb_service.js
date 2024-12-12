// package: pb
// file: bchrpc.proto

var bchrpc_pb = require("./bchrpc_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var bchrpc = (function () {
  function bchrpc() {}
  bchrpc.serviceName = "pb.bchrpc";
  return bchrpc;
}());

bchrpc.GetMempoolInfo = {
  methodName: "GetMempoolInfo",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetMempoolInfoRequest,
  responseType: bchrpc_pb.GetMempoolInfoResponse
};

bchrpc.GetMempool = {
  methodName: "GetMempool",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetMempoolRequest,
  responseType: bchrpc_pb.GetMempoolResponse
};

bchrpc.GetBlockchainInfo = {
  methodName: "GetBlockchainInfo",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetBlockchainInfoRequest,
  responseType: bchrpc_pb.GetBlockchainInfoResponse
};

bchrpc.GetBlockInfo = {
  methodName: "GetBlockInfo",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetBlockInfoRequest,
  responseType: bchrpc_pb.GetBlockInfoResponse
};

bchrpc.GetBlock = {
  methodName: "GetBlock",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetBlockRequest,
  responseType: bchrpc_pb.GetBlockResponse
};

bchrpc.GetRawBlock = {
  methodName: "GetRawBlock",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetRawBlockRequest,
  responseType: bchrpc_pb.GetRawBlockResponse
};

bchrpc.GetBlockFilter = {
  methodName: "GetBlockFilter",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetBlockFilterRequest,
  responseType: bchrpc_pb.GetBlockFilterResponse
};

bchrpc.GetHeaders = {
  methodName: "GetHeaders",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetHeadersRequest,
  responseType: bchrpc_pb.GetHeadersResponse
};

bchrpc.GetTransaction = {
  methodName: "GetTransaction",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetTransactionRequest,
  responseType: bchrpc_pb.GetTransactionResponse
};

bchrpc.GetRawTransaction = {
  methodName: "GetRawTransaction",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetRawTransactionRequest,
  responseType: bchrpc_pb.GetRawTransactionResponse
};

bchrpc.GetAddressTransactions = {
  methodName: "GetAddressTransactions",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetAddressTransactionsRequest,
  responseType: bchrpc_pb.GetAddressTransactionsResponse
};

bchrpc.GetRawAddressTransactions = {
  methodName: "GetRawAddressTransactions",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetRawAddressTransactionsRequest,
  responseType: bchrpc_pb.GetRawAddressTransactionsResponse
};

bchrpc.GetAddressUnspentOutputs = {
  methodName: "GetAddressUnspentOutputs",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetAddressUnspentOutputsRequest,
  responseType: bchrpc_pb.GetAddressUnspentOutputsResponse
};

bchrpc.GetUnspentOutput = {
  methodName: "GetUnspentOutput",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetUnspentOutputRequest,
  responseType: bchrpc_pb.GetUnspentOutputResponse
};

bchrpc.GetMerkleProof = {
  methodName: "GetMerkleProof",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetMerkleProofRequest,
  responseType: bchrpc_pb.GetMerkleProofResponse
};

bchrpc.GetSlpTokenMetadata = {
  methodName: "GetSlpTokenMetadata",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetSlpTokenMetadataRequest,
  responseType: bchrpc_pb.GetSlpTokenMetadataResponse
};

bchrpc.GetSlpParsedScript = {
  methodName: "GetSlpParsedScript",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetSlpParsedScriptRequest,
  responseType: bchrpc_pb.GetSlpParsedScriptResponse
};

bchrpc.GetSlpTrustedValidation = {
  methodName: "GetSlpTrustedValidation",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetSlpTrustedValidationRequest,
  responseType: bchrpc_pb.GetSlpTrustedValidationResponse
};

bchrpc.GetSlpGraphSearch = {
  methodName: "GetSlpGraphSearch",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.GetSlpGraphSearchRequest,
  responseType: bchrpc_pb.GetSlpGraphSearchResponse
};

bchrpc.CheckSlpTransaction = {
  methodName: "CheckSlpTransaction",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.CheckSlpTransactionRequest,
  responseType: bchrpc_pb.CheckSlpTransactionResponse
};

bchrpc.SubmitTransaction = {
  methodName: "SubmitTransaction",
  service: bchrpc,
  requestStream: false,
  responseStream: false,
  requestType: bchrpc_pb.SubmitTransactionRequest,
  responseType: bchrpc_pb.SubmitTransactionResponse
};

bchrpc.SubscribeTransactions = {
  methodName: "SubscribeTransactions",
  service: bchrpc,
  requestStream: false,
  responseStream: true,
  requestType: bchrpc_pb.SubscribeTransactionsRequest,
  responseType: bchrpc_pb.TransactionNotification
};

bchrpc.SubscribeTransactionStream = {
  methodName: "SubscribeTransactionStream",
  service: bchrpc,
  requestStream: true,
  responseStream: true,
  requestType: bchrpc_pb.SubscribeTransactionsRequest,
  responseType: bchrpc_pb.TransactionNotification
};

bchrpc.SubscribeBlocks = {
  methodName: "SubscribeBlocks",
  service: bchrpc,
  requestStream: false,
  responseStream: true,
  requestType: bchrpc_pb.SubscribeBlocksRequest,
  responseType: bchrpc_pb.BlockNotification
};

exports.bchrpc = bchrpc;

function bchrpcClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

bchrpcClient.prototype.getMempoolInfo = function getMempoolInfo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetMempoolInfo, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getMempool = function getMempool(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetMempool, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getBlockchainInfo = function getBlockchainInfo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetBlockchainInfo, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getBlockInfo = function getBlockInfo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetBlockInfo, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getBlock = function getBlock(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetBlock, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getRawBlock = function getRawBlock(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetRawBlock, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getBlockFilter = function getBlockFilter(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetBlockFilter, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getHeaders = function getHeaders(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetHeaders, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getTransaction = function getTransaction(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetTransaction, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getRawTransaction = function getRawTransaction(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetRawTransaction, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getAddressTransactions = function getAddressTransactions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetAddressTransactions, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getRawAddressTransactions = function getRawAddressTransactions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetRawAddressTransactions, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getAddressUnspentOutputs = function getAddressUnspentOutputs(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetAddressUnspentOutputs, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getUnspentOutput = function getUnspentOutput(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetUnspentOutput, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getMerkleProof = function getMerkleProof(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetMerkleProof, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getSlpTokenMetadata = function getSlpTokenMetadata(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetSlpTokenMetadata, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getSlpParsedScript = function getSlpParsedScript(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetSlpParsedScript, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getSlpTrustedValidation = function getSlpTrustedValidation(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetSlpTrustedValidation, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.getSlpGraphSearch = function getSlpGraphSearch(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.GetSlpGraphSearch, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.checkSlpTransaction = function checkSlpTransaction(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.CheckSlpTransaction, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.submitTransaction = function submitTransaction(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(bchrpc.SubmitTransaction, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.subscribeTransactions = function subscribeTransactions(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(bchrpc.SubscribeTransactions, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onMessage: function (responseMessage) {
      listeners.data.forEach(function (handler) {
        handler(responseMessage);
      });
    },
    onEnd: function (status, statusMessage, trailers) {
      listeners.status.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners.end.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners = null;
    }
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    cancel: function () {
      listeners = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.subscribeTransactionStream = function subscribeTransactionStream(metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.client(bchrpc.SubscribeTransactionStream, {
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport
  });
  client.onEnd(function (status, statusMessage, trailers) {
    listeners.status.forEach(function (handler) {
      handler({ code: status, details: statusMessage, metadata: trailers });
    });
    listeners.end.forEach(function (handler) {
      handler({ code: status, details: statusMessage, metadata: trailers });
    });
    listeners = null;
  });
  client.onMessage(function (message) {
    listeners.data.forEach(function (handler) {
      handler(message);
    })
  });
  client.start(metadata);
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    write: function (requestMessage) {
      client.send(requestMessage);
      return this;
    },
    end: function () {
      client.finishSend();
    },
    cancel: function () {
      listeners = null;
      client.close();
    }
  };
};

bchrpcClient.prototype.subscribeBlocks = function subscribeBlocks(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(bchrpc.SubscribeBlocks, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onMessage: function (responseMessage) {
      listeners.data.forEach(function (handler) {
        handler(responseMessage);
      });
    },
    onEnd: function (status, statusMessage, trailers) {
      listeners.status.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners.end.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners = null;
    }
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    cancel: function () {
      listeners = null;
      client.close();
    }
  };
};

exports.bchrpcClient = bchrpcClient;

