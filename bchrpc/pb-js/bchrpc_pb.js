/**
 * @fileoverview
 * @enhanceable
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!

var jspb = require('google-protobuf');
var goog = jspb;
var global = Function('return this')();

goog.exportSymbol('proto.pb.Block', null, global);
goog.exportSymbol('proto.pb.Block.TransactionData', null, global);
goog.exportSymbol('proto.pb.BlockInfo', null, global);
goog.exportSymbol('proto.pb.BlockNotification', null, global);
goog.exportSymbol('proto.pb.BlockNotification.Type', null, global);
goog.exportSymbol('proto.pb.CashToken', null, global);
goog.exportSymbol('proto.pb.CheckSlpTransactionRequest', null, global);
goog.exportSymbol('proto.pb.CheckSlpTransactionResponse', null, global);
goog.exportSymbol('proto.pb.GetAddressTransactionsRequest', null, global);
goog.exportSymbol('proto.pb.GetAddressTransactionsResponse', null, global);
goog.exportSymbol('proto.pb.GetAddressUnspentOutputsRequest', null, global);
goog.exportSymbol('proto.pb.GetAddressUnspentOutputsResponse', null, global);
goog.exportSymbol('proto.pb.GetBlockFilterRequest', null, global);
goog.exportSymbol('proto.pb.GetBlockFilterResponse', null, global);
goog.exportSymbol('proto.pb.GetBlockInfoRequest', null, global);
goog.exportSymbol('proto.pb.GetBlockInfoResponse', null, global);
goog.exportSymbol('proto.pb.GetBlockRequest', null, global);
goog.exportSymbol('proto.pb.GetBlockResponse', null, global);
goog.exportSymbol('proto.pb.GetBlockchainInfoRequest', null, global);
goog.exportSymbol('proto.pb.GetBlockchainInfoResponse', null, global);
goog.exportSymbol('proto.pb.GetBlockchainInfoResponse.BitcoinNet', null, global);
goog.exportSymbol('proto.pb.GetHeadersRequest', null, global);
goog.exportSymbol('proto.pb.GetHeadersResponse', null, global);
goog.exportSymbol('proto.pb.GetMempoolInfoRequest', null, global);
goog.exportSymbol('proto.pb.GetMempoolInfoResponse', null, global);
goog.exportSymbol('proto.pb.GetMempoolRequest', null, global);
goog.exportSymbol('proto.pb.GetMempoolResponse', null, global);
goog.exportSymbol('proto.pb.GetMempoolResponse.TransactionData', null, global);
goog.exportSymbol('proto.pb.GetMerkleProofRequest', null, global);
goog.exportSymbol('proto.pb.GetMerkleProofResponse', null, global);
goog.exportSymbol('proto.pb.GetRawAddressTransactionsRequest', null, global);
goog.exportSymbol('proto.pb.GetRawAddressTransactionsResponse', null, global);
goog.exportSymbol('proto.pb.GetRawBlockRequest', null, global);
goog.exportSymbol('proto.pb.GetRawBlockResponse', null, global);
goog.exportSymbol('proto.pb.GetRawTransactionRequest', null, global);
goog.exportSymbol('proto.pb.GetRawTransactionResponse', null, global);
goog.exportSymbol('proto.pb.GetSlpGraphSearchRequest', null, global);
goog.exportSymbol('proto.pb.GetSlpGraphSearchResponse', null, global);
goog.exportSymbol('proto.pb.GetSlpParsedScriptRequest', null, global);
goog.exportSymbol('proto.pb.GetSlpParsedScriptResponse', null, global);
goog.exportSymbol('proto.pb.GetSlpTokenMetadataRequest', null, global);
goog.exportSymbol('proto.pb.GetSlpTokenMetadataResponse', null, global);
goog.exportSymbol('proto.pb.GetSlpTrustedValidationRequest', null, global);
goog.exportSymbol('proto.pb.GetSlpTrustedValidationRequest.Query', null, global);
goog.exportSymbol('proto.pb.GetSlpTrustedValidationResponse', null, global);
goog.exportSymbol('proto.pb.GetSlpTrustedValidationResponse.ValidityResult', null, global);
goog.exportSymbol('proto.pb.GetTransactionRequest', null, global);
goog.exportSymbol('proto.pb.GetTransactionResponse', null, global);
goog.exportSymbol('proto.pb.GetUnspentOutputRequest', null, global);
goog.exportSymbol('proto.pb.GetUnspentOutputResponse', null, global);
goog.exportSymbol('proto.pb.MempoolTransaction', null, global);
goog.exportSymbol('proto.pb.SlpAction', null, global);
goog.exportSymbol('proto.pb.SlpRequiredBurn', null, global);
goog.exportSymbol('proto.pb.SlpToken', null, global);
goog.exportSymbol('proto.pb.SlpTokenMetadata', null, global);
goog.exportSymbol('proto.pb.SlpTokenMetadata.V1Fungible', null, global);
goog.exportSymbol('proto.pb.SlpTokenMetadata.V1NFT1Child', null, global);
goog.exportSymbol('proto.pb.SlpTokenMetadata.V1NFT1Group', null, global);
goog.exportSymbol('proto.pb.SlpTokenType', null, global);
goog.exportSymbol('proto.pb.SlpTransactionInfo', null, global);
goog.exportSymbol('proto.pb.SlpTransactionInfo.BurnFlags', null, global);
goog.exportSymbol('proto.pb.SlpTransactionInfo.ValidityJudgement', null, global);
goog.exportSymbol('proto.pb.SlpV1GenesisMetadata', null, global);
goog.exportSymbol('proto.pb.SlpV1MintMetadata', null, global);
goog.exportSymbol('proto.pb.SlpV1Nft1ChildGenesisMetadata', null, global);
goog.exportSymbol('proto.pb.SlpV1Nft1ChildSendMetadata', null, global);
goog.exportSymbol('proto.pb.SlpV1SendMetadata', null, global);
goog.exportSymbol('proto.pb.SubmitTransactionRequest', null, global);
goog.exportSymbol('proto.pb.SubmitTransactionResponse', null, global);
goog.exportSymbol('proto.pb.SubscribeBlocksRequest', null, global);
goog.exportSymbol('proto.pb.SubscribeTransactionsRequest', null, global);
goog.exportSymbol('proto.pb.Transaction', null, global);
goog.exportSymbol('proto.pb.Transaction.Input', null, global);
goog.exportSymbol('proto.pb.Transaction.Input.Outpoint', null, global);
goog.exportSymbol('proto.pb.Transaction.Output', null, global);
goog.exportSymbol('proto.pb.TransactionFilter', null, global);
goog.exportSymbol('proto.pb.TransactionNotification', null, global);
goog.exportSymbol('proto.pb.TransactionNotification.Type', null, global);
goog.exportSymbol('proto.pb.UnspentOutput', null, global);

/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetMempoolInfoRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetMempoolInfoRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetMempoolInfoRequest.displayName = 'proto.pb.GetMempoolInfoRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetMempoolInfoRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetMempoolInfoRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetMempoolInfoRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolInfoRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetMempoolInfoRequest}
 */
proto.pb.GetMempoolInfoRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetMempoolInfoRequest;
  return proto.pb.GetMempoolInfoRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetMempoolInfoRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetMempoolInfoRequest}
 */
proto.pb.GetMempoolInfoRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetMempoolInfoRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetMempoolInfoRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetMempoolInfoRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolInfoRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetMempoolInfoResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetMempoolInfoResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetMempoolInfoResponse.displayName = 'proto.pb.GetMempoolInfoResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetMempoolInfoResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetMempoolInfoResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetMempoolInfoResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolInfoResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    size: jspb.Message.getFieldWithDefault(msg, 1, 0),
    bytes: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetMempoolInfoResponse}
 */
proto.pb.GetMempoolInfoResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetMempoolInfoResponse;
  return proto.pb.GetMempoolInfoResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetMempoolInfoResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetMempoolInfoResponse}
 */
proto.pb.GetMempoolInfoResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setSize(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setBytes(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetMempoolInfoResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetMempoolInfoResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetMempoolInfoResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolInfoResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSize();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getBytes();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
};


/**
 * optional uint32 size = 1;
 * @return {number}
 */
proto.pb.GetMempoolInfoResponse.prototype.getSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {number} value */
proto.pb.GetMempoolInfoResponse.prototype.setSize = function(value) {
  jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional uint32 bytes = 2;
 * @return {number}
 */
proto.pb.GetMempoolInfoResponse.prototype.getBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetMempoolInfoResponse.prototype.setBytes = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetMempoolRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetMempoolRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetMempoolRequest.displayName = 'proto.pb.GetMempoolRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetMempoolRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetMempoolRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetMempoolRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    fullTransactions: jspb.Message.getFieldWithDefault(msg, 1, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetMempoolRequest}
 */
proto.pb.GetMempoolRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetMempoolRequest;
  return proto.pb.GetMempoolRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetMempoolRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetMempoolRequest}
 */
proto.pb.GetMempoolRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setFullTransactions(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetMempoolRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetMempoolRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetMempoolRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFullTransactions();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool full_transactions = 1;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetMempoolRequest.prototype.getFullTransactions = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 1, false));
};


/** @param {boolean} value */
proto.pb.GetMempoolRequest.prototype.setFullTransactions = function(value) {
  jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetMempoolResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetMempoolResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetMempoolResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetMempoolResponse.displayName = 'proto.pb.GetMempoolResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetMempoolResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetMempoolResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetMempoolResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetMempoolResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    transactionDataList: jspb.Message.toObjectList(msg.getTransactionDataList(),
    proto.pb.GetMempoolResponse.TransactionData.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetMempoolResponse}
 */
proto.pb.GetMempoolResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetMempoolResponse;
  return proto.pb.GetMempoolResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetMempoolResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetMempoolResponse}
 */
proto.pb.GetMempoolResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.GetMempoolResponse.TransactionData;
      reader.readMessage(value,proto.pb.GetMempoolResponse.TransactionData.deserializeBinaryFromReader);
      msg.addTransactionData(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetMempoolResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetMempoolResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetMempoolResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTransactionDataList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.pb.GetMempoolResponse.TransactionData.serializeBinaryToWriter
    );
  }
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetMempoolResponse.TransactionData = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetMempoolResponse.TransactionData.oneofGroups_);
};
goog.inherits(proto.pb.GetMempoolResponse.TransactionData, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetMempoolResponse.TransactionData.displayName = 'proto.pb.GetMempoolResponse.TransactionData';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetMempoolResponse.TransactionData.oneofGroups_ = [[1,2]];

/**
 * @enum {number}
 */
proto.pb.GetMempoolResponse.TransactionData.TxidsOrTxsCase = {
  TXIDS_OR_TXS_NOT_SET: 0,
  TRANSACTION_HASH: 1,
  TRANSACTION: 2
};

/**
 * @return {proto.pb.GetMempoolResponse.TransactionData.TxidsOrTxsCase}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.getTxidsOrTxsCase = function() {
  return /** @type {proto.pb.GetMempoolResponse.TransactionData.TxidsOrTxsCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetMempoolResponse.TransactionData.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetMempoolResponse.TransactionData.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetMempoolResponse.TransactionData} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolResponse.TransactionData.toObject = function(includeInstance, msg) {
  var f, obj = {
    transactionHash: msg.getTransactionHash_asB64(),
    transaction: (f = msg.getTransaction()) && proto.pb.Transaction.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetMempoolResponse.TransactionData}
 */
proto.pb.GetMempoolResponse.TransactionData.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetMempoolResponse.TransactionData;
  return proto.pb.GetMempoolResponse.TransactionData.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetMempoolResponse.TransactionData} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetMempoolResponse.TransactionData}
 */
proto.pb.GetMempoolResponse.TransactionData.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTransactionHash(value);
      break;
    case 2:
      var value = new proto.pb.Transaction;
      reader.readMessage(value,proto.pb.Transaction.deserializeBinaryFromReader);
      msg.setTransaction(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetMempoolResponse.TransactionData.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetMempoolResponse.TransactionData} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMempoolResponse.TransactionData.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 1));
  if (f != null) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getTransaction();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.pb.Transaction.serializeBinaryToWriter
    );
  }
};


/**
 * optional bytes transaction_hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.getTransactionHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes transaction_hash = 1;
 * This is a type-conversion wrapper around `getTransactionHash()`
 * @return {string}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.getTransactionHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTransactionHash()));
};


/**
 * optional bytes transaction_hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTransactionHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.getTransactionHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTransactionHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetMempoolResponse.TransactionData.prototype.setTransactionHash = function(value) {
  jspb.Message.setOneofField(this, 1, proto.pb.GetMempoolResponse.TransactionData.oneofGroups_[0], value);
};


proto.pb.GetMempoolResponse.TransactionData.prototype.clearTransactionHash = function() {
  jspb.Message.setOneofField(this, 1, proto.pb.GetMempoolResponse.TransactionData.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.hasTransactionHash = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional Transaction transaction = 2;
 * @return {?proto.pb.Transaction}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.getTransaction = function() {
  return /** @type{?proto.pb.Transaction} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction, 2));
};


/** @param {?proto.pb.Transaction|undefined} value */
proto.pb.GetMempoolResponse.TransactionData.prototype.setTransaction = function(value) {
  jspb.Message.setOneofWrapperField(this, 2, proto.pb.GetMempoolResponse.TransactionData.oneofGroups_[0], value);
};


proto.pb.GetMempoolResponse.TransactionData.prototype.clearTransaction = function() {
  this.setTransaction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetMempoolResponse.TransactionData.prototype.hasTransaction = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * repeated TransactionData transaction_data = 1;
 * @return {!Array<!proto.pb.GetMempoolResponse.TransactionData>}
 */
proto.pb.GetMempoolResponse.prototype.getTransactionDataList = function() {
  return /** @type{!Array<!proto.pb.GetMempoolResponse.TransactionData>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.GetMempoolResponse.TransactionData, 1));
};


/** @param {!Array<!proto.pb.GetMempoolResponse.TransactionData>} value */
proto.pb.GetMempoolResponse.prototype.setTransactionDataList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.pb.GetMempoolResponse.TransactionData=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.GetMempoolResponse.TransactionData}
 */
proto.pb.GetMempoolResponse.prototype.addTransactionData = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.pb.GetMempoolResponse.TransactionData, opt_index);
};


proto.pb.GetMempoolResponse.prototype.clearTransactionDataList = function() {
  this.setTransactionDataList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetBlockchainInfoRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetBlockchainInfoRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetBlockchainInfoRequest.displayName = 'proto.pb.GetBlockchainInfoRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetBlockchainInfoRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetBlockchainInfoRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetBlockchainInfoRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockchainInfoRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetBlockchainInfoRequest}
 */
proto.pb.GetBlockchainInfoRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetBlockchainInfoRequest;
  return proto.pb.GetBlockchainInfoRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetBlockchainInfoRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetBlockchainInfoRequest}
 */
proto.pb.GetBlockchainInfoRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetBlockchainInfoRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetBlockchainInfoRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetBlockchainInfoRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockchainInfoRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetBlockchainInfoResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetBlockchainInfoResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetBlockchainInfoResponse.displayName = 'proto.pb.GetBlockchainInfoResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetBlockchainInfoResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetBlockchainInfoResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetBlockchainInfoResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockchainInfoResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    bitcoinNet: jspb.Message.getFieldWithDefault(msg, 1, 0),
    bestHeight: jspb.Message.getFieldWithDefault(msg, 2, 0),
    bestBlockHash: msg.getBestBlockHash_asB64(),
    difficulty: +jspb.Message.getFieldWithDefault(msg, 4, 0.0),
    medianTime: jspb.Message.getFieldWithDefault(msg, 5, 0),
    txIndex: jspb.Message.getFieldWithDefault(msg, 6, false),
    addrIndex: jspb.Message.getFieldWithDefault(msg, 7, false),
    slpIndex: jspb.Message.getFieldWithDefault(msg, 8, false),
    slpGraphsearch: jspb.Message.getFieldWithDefault(msg, 9, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetBlockchainInfoResponse}
 */
proto.pb.GetBlockchainInfoResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetBlockchainInfoResponse;
  return proto.pb.GetBlockchainInfoResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetBlockchainInfoResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetBlockchainInfoResponse}
 */
proto.pb.GetBlockchainInfoResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.pb.GetBlockchainInfoResponse.BitcoinNet} */ (reader.readEnum());
      msg.setBitcoinNet(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setBestHeight(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setBestBlockHash(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setDifficulty(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setMedianTime(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setTxIndex(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAddrIndex(value);
      break;
    case 8:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSlpIndex(value);
      break;
    case 9:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSlpGraphsearch(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetBlockchainInfoResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetBlockchainInfoResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetBlockchainInfoResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockchainInfoResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBitcoinNet();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getBestHeight();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getBestBlockHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getDifficulty();
  if (f !== 0.0) {
    writer.writeDouble(
      4,
      f
    );
  }
  f = message.getMedianTime();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getTxIndex();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getAddrIndex();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getSlpIndex();
  if (f) {
    writer.writeBool(
      8,
      f
    );
  }
  f = message.getSlpGraphsearch();
  if (f) {
    writer.writeBool(
      9,
      f
    );
  }
};


/**
 * @enum {number}
 */
proto.pb.GetBlockchainInfoResponse.BitcoinNet = {
  MAINNET: 0,
  REGTEST: 1,
  TESTNET3: 2,
  SIMNET: 3
};

/**
 * optional BitcoinNet bitcoin_net = 1;
 * @return {!proto.pb.GetBlockchainInfoResponse.BitcoinNet}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getBitcoinNet = function() {
  return /** @type {!proto.pb.GetBlockchainInfoResponse.BitcoinNet} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {!proto.pb.GetBlockchainInfoResponse.BitcoinNet} value */
proto.pb.GetBlockchainInfoResponse.prototype.setBitcoinNet = function(value) {
  jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional int32 best_height = 2;
 * @return {number}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getBestHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetBlockchainInfoResponse.prototype.setBestHeight = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional bytes best_block_hash = 3;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getBestBlockHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes best_block_hash = 3;
 * This is a type-conversion wrapper around `getBestBlockHash()`
 * @return {string}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getBestBlockHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getBestBlockHash()));
};


/**
 * optional bytes best_block_hash = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getBestBlockHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getBestBlockHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getBestBlockHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetBlockchainInfoResponse.prototype.setBestBlockHash = function(value) {
  jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional double difficulty = 4;
 * @return {number}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getDifficulty = function() {
  return /** @type {number} */ (+jspb.Message.getFieldWithDefault(this, 4, 0.0));
};


/** @param {number} value */
proto.pb.GetBlockchainInfoResponse.prototype.setDifficulty = function(value) {
  jspb.Message.setProto3FloatField(this, 4, value);
};


/**
 * optional int64 median_time = 5;
 * @return {number}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getMedianTime = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.GetBlockchainInfoResponse.prototype.setMedianTime = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bool tx_index = 6;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getTxIndex = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 6, false));
};


/** @param {boolean} value */
proto.pb.GetBlockchainInfoResponse.prototype.setTxIndex = function(value) {
  jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional bool addr_index = 7;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getAddrIndex = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 7, false));
};


/** @param {boolean} value */
proto.pb.GetBlockchainInfoResponse.prototype.setAddrIndex = function(value) {
  jspb.Message.setProto3BooleanField(this, 7, value);
};


/**
 * optional bool slp_index = 8;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getSlpIndex = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 8, false));
};


/** @param {boolean} value */
proto.pb.GetBlockchainInfoResponse.prototype.setSlpIndex = function(value) {
  jspb.Message.setProto3BooleanField(this, 8, value);
};


/**
 * optional bool slp_graphsearch = 9;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetBlockchainInfoResponse.prototype.getSlpGraphsearch = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 9, false));
};


/** @param {boolean} value */
proto.pb.GetBlockchainInfoResponse.prototype.setSlpGraphsearch = function(value) {
  jspb.Message.setProto3BooleanField(this, 9, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetBlockInfoRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetBlockInfoRequest.oneofGroups_);
};
goog.inherits(proto.pb.GetBlockInfoRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetBlockInfoRequest.displayName = 'proto.pb.GetBlockInfoRequest';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetBlockInfoRequest.oneofGroups_ = [[1,2]];

/**
 * @enum {number}
 */
proto.pb.GetBlockInfoRequest.HashOrHeightCase = {
  HASH_OR_HEIGHT_NOT_SET: 0,
  HASH: 1,
  HEIGHT: 2
};

/**
 * @return {proto.pb.GetBlockInfoRequest.HashOrHeightCase}
 */
proto.pb.GetBlockInfoRequest.prototype.getHashOrHeightCase = function() {
  return /** @type {proto.pb.GetBlockInfoRequest.HashOrHeightCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetBlockInfoRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetBlockInfoRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetBlockInfoRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetBlockInfoRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockInfoRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    height: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetBlockInfoRequest}
 */
proto.pb.GetBlockInfoRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetBlockInfoRequest;
  return proto.pb.GetBlockInfoRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetBlockInfoRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetBlockInfoRequest}
 */
proto.pb.GetBlockInfoRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHeight(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetBlockInfoRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetBlockInfoRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetBlockInfoRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockInfoRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 1));
  if (f != null) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = /** @type {number} */ (jspb.Message.getField(message, 2));
  if (f != null) {
    writer.writeInt32(
      2,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetBlockInfoRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetBlockInfoRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetBlockInfoRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetBlockInfoRequest.prototype.setHash = function(value) {
  jspb.Message.setOneofField(this, 1, proto.pb.GetBlockInfoRequest.oneofGroups_[0], value);
};


proto.pb.GetBlockInfoRequest.prototype.clearHash = function() {
  jspb.Message.setOneofField(this, 1, proto.pb.GetBlockInfoRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetBlockInfoRequest.prototype.hasHash = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int32 height = 2;
 * @return {number}
 */
proto.pb.GetBlockInfoRequest.prototype.getHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetBlockInfoRequest.prototype.setHeight = function(value) {
  jspb.Message.setOneofField(this, 2, proto.pb.GetBlockInfoRequest.oneofGroups_[0], value);
};


proto.pb.GetBlockInfoRequest.prototype.clearHeight = function() {
  jspb.Message.setOneofField(this, 2, proto.pb.GetBlockInfoRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetBlockInfoRequest.prototype.hasHeight = function() {
  return jspb.Message.getField(this, 2) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetBlockInfoResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetBlockInfoResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetBlockInfoResponse.displayName = 'proto.pb.GetBlockInfoResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetBlockInfoResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetBlockInfoResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetBlockInfoResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockInfoResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    info: (f = msg.getInfo()) && proto.pb.BlockInfo.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetBlockInfoResponse}
 */
proto.pb.GetBlockInfoResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetBlockInfoResponse;
  return proto.pb.GetBlockInfoResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetBlockInfoResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetBlockInfoResponse}
 */
proto.pb.GetBlockInfoResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.BlockInfo;
      reader.readMessage(value,proto.pb.BlockInfo.deserializeBinaryFromReader);
      msg.setInfo(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetBlockInfoResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetBlockInfoResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetBlockInfoResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockInfoResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getInfo();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.BlockInfo.serializeBinaryToWriter
    );
  }
};


/**
 * optional BlockInfo info = 1;
 * @return {?proto.pb.BlockInfo}
 */
proto.pb.GetBlockInfoResponse.prototype.getInfo = function() {
  return /** @type{?proto.pb.BlockInfo} */ (
    jspb.Message.getWrapperField(this, proto.pb.BlockInfo, 1));
};


/** @param {?proto.pb.BlockInfo|undefined} value */
proto.pb.GetBlockInfoResponse.prototype.setInfo = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.GetBlockInfoResponse.prototype.clearInfo = function() {
  this.setInfo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetBlockInfoResponse.prototype.hasInfo = function() {
  return jspb.Message.getField(this, 1) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetBlockRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetBlockRequest.oneofGroups_);
};
goog.inherits(proto.pb.GetBlockRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetBlockRequest.displayName = 'proto.pb.GetBlockRequest';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetBlockRequest.oneofGroups_ = [[1,2]];

/**
 * @enum {number}
 */
proto.pb.GetBlockRequest.HashOrHeightCase = {
  HASH_OR_HEIGHT_NOT_SET: 0,
  HASH: 1,
  HEIGHT: 2
};

/**
 * @return {proto.pb.GetBlockRequest.HashOrHeightCase}
 */
proto.pb.GetBlockRequest.prototype.getHashOrHeightCase = function() {
  return /** @type {proto.pb.GetBlockRequest.HashOrHeightCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetBlockRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetBlockRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetBlockRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetBlockRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    height: jspb.Message.getFieldWithDefault(msg, 2, 0),
    fullTransactions: jspb.Message.getFieldWithDefault(msg, 3, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetBlockRequest}
 */
proto.pb.GetBlockRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetBlockRequest;
  return proto.pb.GetBlockRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetBlockRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetBlockRequest}
 */
proto.pb.GetBlockRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHeight(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setFullTransactions(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetBlockRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetBlockRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetBlockRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 1));
  if (f != null) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = /** @type {number} */ (jspb.Message.getField(message, 2));
  if (f != null) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getFullTransactions();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetBlockRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetBlockRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetBlockRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetBlockRequest.prototype.setHash = function(value) {
  jspb.Message.setOneofField(this, 1, proto.pb.GetBlockRequest.oneofGroups_[0], value);
};


proto.pb.GetBlockRequest.prototype.clearHash = function() {
  jspb.Message.setOneofField(this, 1, proto.pb.GetBlockRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetBlockRequest.prototype.hasHash = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int32 height = 2;
 * @return {number}
 */
proto.pb.GetBlockRequest.prototype.getHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetBlockRequest.prototype.setHeight = function(value) {
  jspb.Message.setOneofField(this, 2, proto.pb.GetBlockRequest.oneofGroups_[0], value);
};


proto.pb.GetBlockRequest.prototype.clearHeight = function() {
  jspb.Message.setOneofField(this, 2, proto.pb.GetBlockRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetBlockRequest.prototype.hasHeight = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool full_transactions = 3;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetBlockRequest.prototype.getFullTransactions = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 3, false));
};


/** @param {boolean} value */
proto.pb.GetBlockRequest.prototype.setFullTransactions = function(value) {
  jspb.Message.setProto3BooleanField(this, 3, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetBlockResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetBlockResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetBlockResponse.displayName = 'proto.pb.GetBlockResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetBlockResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetBlockResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetBlockResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    block: (f = msg.getBlock()) && proto.pb.Block.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetBlockResponse}
 */
proto.pb.GetBlockResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetBlockResponse;
  return proto.pb.GetBlockResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetBlockResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetBlockResponse}
 */
proto.pb.GetBlockResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.Block;
      reader.readMessage(value,proto.pb.Block.deserializeBinaryFromReader);
      msg.setBlock(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetBlockResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetBlockResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetBlockResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBlock();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.Block.serializeBinaryToWriter
    );
  }
};


/**
 * optional Block block = 1;
 * @return {?proto.pb.Block}
 */
proto.pb.GetBlockResponse.prototype.getBlock = function() {
  return /** @type{?proto.pb.Block} */ (
    jspb.Message.getWrapperField(this, proto.pb.Block, 1));
};


/** @param {?proto.pb.Block|undefined} value */
proto.pb.GetBlockResponse.prototype.setBlock = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.GetBlockResponse.prototype.clearBlock = function() {
  this.setBlock(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetBlockResponse.prototype.hasBlock = function() {
  return jspb.Message.getField(this, 1) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetRawBlockRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetRawBlockRequest.oneofGroups_);
};
goog.inherits(proto.pb.GetRawBlockRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetRawBlockRequest.displayName = 'proto.pb.GetRawBlockRequest';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetRawBlockRequest.oneofGroups_ = [[1,2]];

/**
 * @enum {number}
 */
proto.pb.GetRawBlockRequest.HashOrHeightCase = {
  HASH_OR_HEIGHT_NOT_SET: 0,
  HASH: 1,
  HEIGHT: 2
};

/**
 * @return {proto.pb.GetRawBlockRequest.HashOrHeightCase}
 */
proto.pb.GetRawBlockRequest.prototype.getHashOrHeightCase = function() {
  return /** @type {proto.pb.GetRawBlockRequest.HashOrHeightCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetRawBlockRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetRawBlockRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetRawBlockRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetRawBlockRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawBlockRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    height: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetRawBlockRequest}
 */
proto.pb.GetRawBlockRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetRawBlockRequest;
  return proto.pb.GetRawBlockRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetRawBlockRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetRawBlockRequest}
 */
proto.pb.GetRawBlockRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHeight(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetRawBlockRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetRawBlockRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetRawBlockRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawBlockRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 1));
  if (f != null) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = /** @type {number} */ (jspb.Message.getField(message, 2));
  if (f != null) {
    writer.writeInt32(
      2,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetRawBlockRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetRawBlockRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetRawBlockRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetRawBlockRequest.prototype.setHash = function(value) {
  jspb.Message.setOneofField(this, 1, proto.pb.GetRawBlockRequest.oneofGroups_[0], value);
};


proto.pb.GetRawBlockRequest.prototype.clearHash = function() {
  jspb.Message.setOneofField(this, 1, proto.pb.GetRawBlockRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetRawBlockRequest.prototype.hasHash = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int32 height = 2;
 * @return {number}
 */
proto.pb.GetRawBlockRequest.prototype.getHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetRawBlockRequest.prototype.setHeight = function(value) {
  jspb.Message.setOneofField(this, 2, proto.pb.GetRawBlockRequest.oneofGroups_[0], value);
};


proto.pb.GetRawBlockRequest.prototype.clearHeight = function() {
  jspb.Message.setOneofField(this, 2, proto.pb.GetRawBlockRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetRawBlockRequest.prototype.hasHeight = function() {
  return jspb.Message.getField(this, 2) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetRawBlockResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetRawBlockResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetRawBlockResponse.displayName = 'proto.pb.GetRawBlockResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetRawBlockResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetRawBlockResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetRawBlockResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawBlockResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    block: msg.getBlock_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetRawBlockResponse}
 */
proto.pb.GetRawBlockResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetRawBlockResponse;
  return proto.pb.GetRawBlockResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetRawBlockResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetRawBlockResponse}
 */
proto.pb.GetRawBlockResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setBlock(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetRawBlockResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetRawBlockResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetRawBlockResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawBlockResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBlock_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes block = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetRawBlockResponse.prototype.getBlock = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes block = 1;
 * This is a type-conversion wrapper around `getBlock()`
 * @return {string}
 */
proto.pb.GetRawBlockResponse.prototype.getBlock_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getBlock()));
};


/**
 * optional bytes block = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getBlock()`
 * @return {!Uint8Array}
 */
proto.pb.GetRawBlockResponse.prototype.getBlock_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getBlock()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetRawBlockResponse.prototype.setBlock = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetBlockFilterRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetBlockFilterRequest.oneofGroups_);
};
goog.inherits(proto.pb.GetBlockFilterRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetBlockFilterRequest.displayName = 'proto.pb.GetBlockFilterRequest';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetBlockFilterRequest.oneofGroups_ = [[1,2]];

/**
 * @enum {number}
 */
proto.pb.GetBlockFilterRequest.HashOrHeightCase = {
  HASH_OR_HEIGHT_NOT_SET: 0,
  HASH: 1,
  HEIGHT: 2
};

/**
 * @return {proto.pb.GetBlockFilterRequest.HashOrHeightCase}
 */
proto.pb.GetBlockFilterRequest.prototype.getHashOrHeightCase = function() {
  return /** @type {proto.pb.GetBlockFilterRequest.HashOrHeightCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetBlockFilterRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetBlockFilterRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetBlockFilterRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetBlockFilterRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockFilterRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    height: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetBlockFilterRequest}
 */
proto.pb.GetBlockFilterRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetBlockFilterRequest;
  return proto.pb.GetBlockFilterRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetBlockFilterRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetBlockFilterRequest}
 */
proto.pb.GetBlockFilterRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHeight(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetBlockFilterRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetBlockFilterRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetBlockFilterRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockFilterRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 1));
  if (f != null) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = /** @type {number} */ (jspb.Message.getField(message, 2));
  if (f != null) {
    writer.writeInt32(
      2,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetBlockFilterRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetBlockFilterRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetBlockFilterRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetBlockFilterRequest.prototype.setHash = function(value) {
  jspb.Message.setOneofField(this, 1, proto.pb.GetBlockFilterRequest.oneofGroups_[0], value);
};


proto.pb.GetBlockFilterRequest.prototype.clearHash = function() {
  jspb.Message.setOneofField(this, 1, proto.pb.GetBlockFilterRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetBlockFilterRequest.prototype.hasHash = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int32 height = 2;
 * @return {number}
 */
proto.pb.GetBlockFilterRequest.prototype.getHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetBlockFilterRequest.prototype.setHeight = function(value) {
  jspb.Message.setOneofField(this, 2, proto.pb.GetBlockFilterRequest.oneofGroups_[0], value);
};


proto.pb.GetBlockFilterRequest.prototype.clearHeight = function() {
  jspb.Message.setOneofField(this, 2, proto.pb.GetBlockFilterRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetBlockFilterRequest.prototype.hasHeight = function() {
  return jspb.Message.getField(this, 2) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetBlockFilterResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetBlockFilterResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetBlockFilterResponse.displayName = 'proto.pb.GetBlockFilterResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetBlockFilterResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetBlockFilterResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetBlockFilterResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockFilterResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    filter: msg.getFilter_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetBlockFilterResponse}
 */
proto.pb.GetBlockFilterResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetBlockFilterResponse;
  return proto.pb.GetBlockFilterResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetBlockFilterResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetBlockFilterResponse}
 */
proto.pb.GetBlockFilterResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setFilter(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetBlockFilterResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetBlockFilterResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetBlockFilterResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetBlockFilterResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFilter_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes filter = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetBlockFilterResponse.prototype.getFilter = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes filter = 1;
 * This is a type-conversion wrapper around `getFilter()`
 * @return {string}
 */
proto.pb.GetBlockFilterResponse.prototype.getFilter_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getFilter()));
};


/**
 * optional bytes filter = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getFilter()`
 * @return {!Uint8Array}
 */
proto.pb.GetBlockFilterResponse.prototype.getFilter_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getFilter()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetBlockFilterResponse.prototype.setFilter = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetHeadersRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetHeadersRequest.repeatedFields_, null);
};
goog.inherits(proto.pb.GetHeadersRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetHeadersRequest.displayName = 'proto.pb.GetHeadersRequest';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetHeadersRequest.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetHeadersRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetHeadersRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetHeadersRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetHeadersRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    blockLocatorHashesList: msg.getBlockLocatorHashesList_asB64(),
    stopHash: msg.getStopHash_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetHeadersRequest}
 */
proto.pb.GetHeadersRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetHeadersRequest;
  return proto.pb.GetHeadersRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetHeadersRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetHeadersRequest}
 */
proto.pb.GetHeadersRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addBlockLocatorHashes(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setStopHash(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetHeadersRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetHeadersRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetHeadersRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetHeadersRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBlockLocatorHashesList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      1,
      f
    );
  }
  f = message.getStopHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
};


/**
 * repeated bytes block_locator_hashes = 1;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.GetHeadersRequest.prototype.getBlockLocatorHashesList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * repeated bytes block_locator_hashes = 1;
 * This is a type-conversion wrapper around `getBlockLocatorHashesList()`
 * @return {!Array<string>}
 */
proto.pb.GetHeadersRequest.prototype.getBlockLocatorHashesList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getBlockLocatorHashesList()));
};


/**
 * repeated bytes block_locator_hashes = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getBlockLocatorHashesList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.GetHeadersRequest.prototype.getBlockLocatorHashesList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getBlockLocatorHashesList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.GetHeadersRequest.prototype.setBlockLocatorHashesList = function(value) {
  jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.GetHeadersRequest.prototype.addBlockLocatorHashes = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


proto.pb.GetHeadersRequest.prototype.clearBlockLocatorHashesList = function() {
  this.setBlockLocatorHashesList([]);
};


/**
 * optional bytes stop_hash = 2;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetHeadersRequest.prototype.getStopHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes stop_hash = 2;
 * This is a type-conversion wrapper around `getStopHash()`
 * @return {string}
 */
proto.pb.GetHeadersRequest.prototype.getStopHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getStopHash()));
};


/**
 * optional bytes stop_hash = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getStopHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetHeadersRequest.prototype.getStopHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getStopHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetHeadersRequest.prototype.setStopHash = function(value) {
  jspb.Message.setProto3BytesField(this, 2, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetHeadersResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetHeadersResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetHeadersResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetHeadersResponse.displayName = 'proto.pb.GetHeadersResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetHeadersResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetHeadersResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetHeadersResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetHeadersResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetHeadersResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    headersList: jspb.Message.toObjectList(msg.getHeadersList(),
    proto.pb.BlockInfo.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetHeadersResponse}
 */
proto.pb.GetHeadersResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetHeadersResponse;
  return proto.pb.GetHeadersResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetHeadersResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetHeadersResponse}
 */
proto.pb.GetHeadersResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.BlockInfo;
      reader.readMessage(value,proto.pb.BlockInfo.deserializeBinaryFromReader);
      msg.addHeaders(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetHeadersResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetHeadersResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetHeadersResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetHeadersResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHeadersList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.pb.BlockInfo.serializeBinaryToWriter
    );
  }
};


/**
 * repeated BlockInfo headers = 1;
 * @return {!Array<!proto.pb.BlockInfo>}
 */
proto.pb.GetHeadersResponse.prototype.getHeadersList = function() {
  return /** @type{!Array<!proto.pb.BlockInfo>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.BlockInfo, 1));
};


/** @param {!Array<!proto.pb.BlockInfo>} value */
proto.pb.GetHeadersResponse.prototype.setHeadersList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.pb.BlockInfo=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.BlockInfo}
 */
proto.pb.GetHeadersResponse.prototype.addHeaders = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.pb.BlockInfo, opt_index);
};


proto.pb.GetHeadersResponse.prototype.clearHeadersList = function() {
  this.setHeadersList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetTransactionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetTransactionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetTransactionRequest.displayName = 'proto.pb.GetTransactionRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetTransactionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetTransactionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetTransactionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetTransactionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    includeTokenMetadata: jspb.Message.getFieldWithDefault(msg, 2, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetTransactionRequest}
 */
proto.pb.GetTransactionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetTransactionRequest;
  return proto.pb.GetTransactionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetTransactionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetTransactionRequest}
 */
proto.pb.GetTransactionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeTokenMetadata(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetTransactionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetTransactionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetTransactionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetTransactionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getIncludeTokenMetadata();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetTransactionRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetTransactionRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetTransactionRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetTransactionRequest.prototype.setHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional bool include_token_metadata = 2;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetTransactionRequest.prototype.getIncludeTokenMetadata = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 2, false));
};


/** @param {boolean} value */
proto.pb.GetTransactionRequest.prototype.setIncludeTokenMetadata = function(value) {
  jspb.Message.setProto3BooleanField(this, 2, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetTransactionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetTransactionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetTransactionResponse.displayName = 'proto.pb.GetTransactionResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetTransactionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetTransactionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetTransactionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetTransactionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    transaction: (f = msg.getTransaction()) && proto.pb.Transaction.toObject(includeInstance, f),
    tokenMetadata: (f = msg.getTokenMetadata()) && proto.pb.SlpTokenMetadata.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetTransactionResponse}
 */
proto.pb.GetTransactionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetTransactionResponse;
  return proto.pb.GetTransactionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetTransactionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetTransactionResponse}
 */
proto.pb.GetTransactionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.Transaction;
      reader.readMessage(value,proto.pb.Transaction.deserializeBinaryFromReader);
      msg.setTransaction(value);
      break;
    case 2:
      var value = new proto.pb.SlpTokenMetadata;
      reader.readMessage(value,proto.pb.SlpTokenMetadata.deserializeBinaryFromReader);
      msg.setTokenMetadata(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetTransactionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetTransactionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetTransactionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetTransactionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTransaction();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.Transaction.serializeBinaryToWriter
    );
  }
  f = message.getTokenMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.pb.SlpTokenMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional Transaction transaction = 1;
 * @return {?proto.pb.Transaction}
 */
proto.pb.GetTransactionResponse.prototype.getTransaction = function() {
  return /** @type{?proto.pb.Transaction} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction, 1));
};


/** @param {?proto.pb.Transaction|undefined} value */
proto.pb.GetTransactionResponse.prototype.setTransaction = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.GetTransactionResponse.prototype.clearTransaction = function() {
  this.setTransaction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetTransactionResponse.prototype.hasTransaction = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional SlpTokenMetadata token_metadata = 2;
 * @return {?proto.pb.SlpTokenMetadata}
 */
proto.pb.GetTransactionResponse.prototype.getTokenMetadata = function() {
  return /** @type{?proto.pb.SlpTokenMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpTokenMetadata, 2));
};


/** @param {?proto.pb.SlpTokenMetadata|undefined} value */
proto.pb.GetTransactionResponse.prototype.setTokenMetadata = function(value) {
  jspb.Message.setWrapperField(this, 2, value);
};


proto.pb.GetTransactionResponse.prototype.clearTokenMetadata = function() {
  this.setTokenMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetTransactionResponse.prototype.hasTokenMetadata = function() {
  return jspb.Message.getField(this, 2) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetRawTransactionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetRawTransactionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetRawTransactionRequest.displayName = 'proto.pb.GetRawTransactionRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetRawTransactionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetRawTransactionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetRawTransactionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawTransactionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetRawTransactionRequest}
 */
proto.pb.GetRawTransactionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetRawTransactionRequest;
  return proto.pb.GetRawTransactionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetRawTransactionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetRawTransactionRequest}
 */
proto.pb.GetRawTransactionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetRawTransactionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetRawTransactionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetRawTransactionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawTransactionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetRawTransactionRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetRawTransactionRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetRawTransactionRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetRawTransactionRequest.prototype.setHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetRawTransactionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetRawTransactionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetRawTransactionResponse.displayName = 'proto.pb.GetRawTransactionResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetRawTransactionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetRawTransactionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetRawTransactionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawTransactionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    transaction: msg.getTransaction_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetRawTransactionResponse}
 */
proto.pb.GetRawTransactionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetRawTransactionResponse;
  return proto.pb.GetRawTransactionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetRawTransactionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetRawTransactionResponse}
 */
proto.pb.GetRawTransactionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTransaction(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetRawTransactionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetRawTransactionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetRawTransactionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawTransactionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTransaction_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes transaction = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetRawTransactionResponse.prototype.getTransaction = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes transaction = 1;
 * This is a type-conversion wrapper around `getTransaction()`
 * @return {string}
 */
proto.pb.GetRawTransactionResponse.prototype.getTransaction_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTransaction()));
};


/**
 * optional bytes transaction = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTransaction()`
 * @return {!Uint8Array}
 */
proto.pb.GetRawTransactionResponse.prototype.getTransaction_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTransaction()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetRawTransactionResponse.prototype.setTransaction = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetAddressTransactionsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetAddressTransactionsRequest.oneofGroups_);
};
goog.inherits(proto.pb.GetAddressTransactionsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetAddressTransactionsRequest.displayName = 'proto.pb.GetAddressTransactionsRequest';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetAddressTransactionsRequest.oneofGroups_ = [[4,5]];

/**
 * @enum {number}
 */
proto.pb.GetAddressTransactionsRequest.StartBlockCase = {
  START_BLOCK_NOT_SET: 0,
  HASH: 4,
  HEIGHT: 5
};

/**
 * @return {proto.pb.GetAddressTransactionsRequest.StartBlockCase}
 */
proto.pb.GetAddressTransactionsRequest.prototype.getStartBlockCase = function() {
  return /** @type {proto.pb.GetAddressTransactionsRequest.StartBlockCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetAddressTransactionsRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetAddressTransactionsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetAddressTransactionsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetAddressTransactionsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetAddressTransactionsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    address: jspb.Message.getFieldWithDefault(msg, 1, ""),
    nbSkip: jspb.Message.getFieldWithDefault(msg, 2, 0),
    nbFetch: jspb.Message.getFieldWithDefault(msg, 3, 0),
    hash: msg.getHash_asB64(),
    height: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetAddressTransactionsRequest}
 */
proto.pb.GetAddressTransactionsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetAddressTransactionsRequest;
  return proto.pb.GetAddressTransactionsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetAddressTransactionsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetAddressTransactionsRequest}
 */
proto.pb.GetAddressTransactionsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAddress(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setNbSkip(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setNbFetch(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHeight(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetAddressTransactionsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetAddressTransactionsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetAddressTransactionsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetAddressTransactionsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAddress();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNbSkip();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getNbFetch();
  if (f !== 0) {
    writer.writeUint32(
      3,
      f
    );
  }
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 4));
  if (f != null) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = /** @type {number} */ (jspb.Message.getField(message, 5));
  if (f != null) {
    writer.writeInt32(
      5,
      f
    );
  }
};


/**
 * optional string address = 1;
 * @return {string}
 */
proto.pb.GetAddressTransactionsRequest.prototype.getAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/** @param {string} value */
proto.pb.GetAddressTransactionsRequest.prototype.setAddress = function(value) {
  jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional uint32 nb_skip = 2;
 * @return {number}
 */
proto.pb.GetAddressTransactionsRequest.prototype.getNbSkip = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetAddressTransactionsRequest.prototype.setNbSkip = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional uint32 nb_fetch = 3;
 * @return {number}
 */
proto.pb.GetAddressTransactionsRequest.prototype.getNbFetch = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {number} value */
proto.pb.GetAddressTransactionsRequest.prototype.setNbFetch = function(value) {
  jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional bytes hash = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetAddressTransactionsRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes hash = 4;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetAddressTransactionsRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetAddressTransactionsRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetAddressTransactionsRequest.prototype.setHash = function(value) {
  jspb.Message.setOneofField(this, 4, proto.pb.GetAddressTransactionsRequest.oneofGroups_[0], value);
};


proto.pb.GetAddressTransactionsRequest.prototype.clearHash = function() {
  jspb.Message.setOneofField(this, 4, proto.pb.GetAddressTransactionsRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetAddressTransactionsRequest.prototype.hasHash = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional int32 height = 5;
 * @return {number}
 */
proto.pb.GetAddressTransactionsRequest.prototype.getHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.GetAddressTransactionsRequest.prototype.setHeight = function(value) {
  jspb.Message.setOneofField(this, 5, proto.pb.GetAddressTransactionsRequest.oneofGroups_[0], value);
};


proto.pb.GetAddressTransactionsRequest.prototype.clearHeight = function() {
  jspb.Message.setOneofField(this, 5, proto.pb.GetAddressTransactionsRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetAddressTransactionsRequest.prototype.hasHeight = function() {
  return jspb.Message.getField(this, 5) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetAddressTransactionsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetAddressTransactionsResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetAddressTransactionsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetAddressTransactionsResponse.displayName = 'proto.pb.GetAddressTransactionsResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetAddressTransactionsResponse.repeatedFields_ = [1,2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetAddressTransactionsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetAddressTransactionsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetAddressTransactionsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetAddressTransactionsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    confirmedTransactionsList: jspb.Message.toObjectList(msg.getConfirmedTransactionsList(),
    proto.pb.Transaction.toObject, includeInstance),
    unconfirmedTransactionsList: jspb.Message.toObjectList(msg.getUnconfirmedTransactionsList(),
    proto.pb.MempoolTransaction.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetAddressTransactionsResponse}
 */
proto.pb.GetAddressTransactionsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetAddressTransactionsResponse;
  return proto.pb.GetAddressTransactionsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetAddressTransactionsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetAddressTransactionsResponse}
 */
proto.pb.GetAddressTransactionsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.Transaction;
      reader.readMessage(value,proto.pb.Transaction.deserializeBinaryFromReader);
      msg.addConfirmedTransactions(value);
      break;
    case 2:
      var value = new proto.pb.MempoolTransaction;
      reader.readMessage(value,proto.pb.MempoolTransaction.deserializeBinaryFromReader);
      msg.addUnconfirmedTransactions(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetAddressTransactionsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetAddressTransactionsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetAddressTransactionsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetAddressTransactionsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getConfirmedTransactionsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.pb.Transaction.serializeBinaryToWriter
    );
  }
  f = message.getUnconfirmedTransactionsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.pb.MempoolTransaction.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Transaction confirmed_transactions = 1;
 * @return {!Array<!proto.pb.Transaction>}
 */
proto.pb.GetAddressTransactionsResponse.prototype.getConfirmedTransactionsList = function() {
  return /** @type{!Array<!proto.pb.Transaction>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.Transaction, 1));
};


/** @param {!Array<!proto.pb.Transaction>} value */
proto.pb.GetAddressTransactionsResponse.prototype.setConfirmedTransactionsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.pb.Transaction=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.Transaction}
 */
proto.pb.GetAddressTransactionsResponse.prototype.addConfirmedTransactions = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.pb.Transaction, opt_index);
};


proto.pb.GetAddressTransactionsResponse.prototype.clearConfirmedTransactionsList = function() {
  this.setConfirmedTransactionsList([]);
};


/**
 * repeated MempoolTransaction unconfirmed_transactions = 2;
 * @return {!Array<!proto.pb.MempoolTransaction>}
 */
proto.pb.GetAddressTransactionsResponse.prototype.getUnconfirmedTransactionsList = function() {
  return /** @type{!Array<!proto.pb.MempoolTransaction>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.MempoolTransaction, 2));
};


/** @param {!Array<!proto.pb.MempoolTransaction>} value */
proto.pb.GetAddressTransactionsResponse.prototype.setUnconfirmedTransactionsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.pb.MempoolTransaction=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.MempoolTransaction}
 */
proto.pb.GetAddressTransactionsResponse.prototype.addUnconfirmedTransactions = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.pb.MempoolTransaction, opt_index);
};


proto.pb.GetAddressTransactionsResponse.prototype.clearUnconfirmedTransactionsList = function() {
  this.setUnconfirmedTransactionsList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetRawAddressTransactionsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetRawAddressTransactionsRequest.oneofGroups_);
};
goog.inherits(proto.pb.GetRawAddressTransactionsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetRawAddressTransactionsRequest.displayName = 'proto.pb.GetRawAddressTransactionsRequest';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetRawAddressTransactionsRequest.oneofGroups_ = [[4,5]];

/**
 * @enum {number}
 */
proto.pb.GetRawAddressTransactionsRequest.StartBlockCase = {
  START_BLOCK_NOT_SET: 0,
  HASH: 4,
  HEIGHT: 5
};

/**
 * @return {proto.pb.GetRawAddressTransactionsRequest.StartBlockCase}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.getStartBlockCase = function() {
  return /** @type {proto.pb.GetRawAddressTransactionsRequest.StartBlockCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetRawAddressTransactionsRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetRawAddressTransactionsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetRawAddressTransactionsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawAddressTransactionsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    address: jspb.Message.getFieldWithDefault(msg, 1, ""),
    nbSkip: jspb.Message.getFieldWithDefault(msg, 2, 0),
    nbFetch: jspb.Message.getFieldWithDefault(msg, 3, 0),
    hash: msg.getHash_asB64(),
    height: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetRawAddressTransactionsRequest}
 */
proto.pb.GetRawAddressTransactionsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetRawAddressTransactionsRequest;
  return proto.pb.GetRawAddressTransactionsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetRawAddressTransactionsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetRawAddressTransactionsRequest}
 */
proto.pb.GetRawAddressTransactionsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAddress(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setNbSkip(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setNbFetch(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHeight(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetRawAddressTransactionsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetRawAddressTransactionsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawAddressTransactionsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAddress();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNbSkip();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getNbFetch();
  if (f !== 0) {
    writer.writeUint32(
      3,
      f
    );
  }
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 4));
  if (f != null) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = /** @type {number} */ (jspb.Message.getField(message, 5));
  if (f != null) {
    writer.writeInt32(
      5,
      f
    );
  }
};


/**
 * optional string address = 1;
 * @return {string}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.getAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/** @param {string} value */
proto.pb.GetRawAddressTransactionsRequest.prototype.setAddress = function(value) {
  jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional uint32 nb_skip = 2;
 * @return {number}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.getNbSkip = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetRawAddressTransactionsRequest.prototype.setNbSkip = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional uint32 nb_fetch = 3;
 * @return {number}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.getNbFetch = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {number} value */
proto.pb.GetRawAddressTransactionsRequest.prototype.setNbFetch = function(value) {
  jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional bytes hash = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes hash = 4;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetRawAddressTransactionsRequest.prototype.setHash = function(value) {
  jspb.Message.setOneofField(this, 4, proto.pb.GetRawAddressTransactionsRequest.oneofGroups_[0], value);
};


proto.pb.GetRawAddressTransactionsRequest.prototype.clearHash = function() {
  jspb.Message.setOneofField(this, 4, proto.pb.GetRawAddressTransactionsRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.hasHash = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional int32 height = 5;
 * @return {number}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.getHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.GetRawAddressTransactionsRequest.prototype.setHeight = function(value) {
  jspb.Message.setOneofField(this, 5, proto.pb.GetRawAddressTransactionsRequest.oneofGroups_[0], value);
};


proto.pb.GetRawAddressTransactionsRequest.prototype.clearHeight = function() {
  jspb.Message.setOneofField(this, 5, proto.pb.GetRawAddressTransactionsRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetRawAddressTransactionsRequest.prototype.hasHeight = function() {
  return jspb.Message.getField(this, 5) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetRawAddressTransactionsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetRawAddressTransactionsResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetRawAddressTransactionsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetRawAddressTransactionsResponse.displayName = 'proto.pb.GetRawAddressTransactionsResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetRawAddressTransactionsResponse.repeatedFields_ = [1,2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetRawAddressTransactionsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetRawAddressTransactionsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawAddressTransactionsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    confirmedTransactionsList: msg.getConfirmedTransactionsList_asB64(),
    unconfirmedTransactionsList: msg.getUnconfirmedTransactionsList_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetRawAddressTransactionsResponse}
 */
proto.pb.GetRawAddressTransactionsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetRawAddressTransactionsResponse;
  return proto.pb.GetRawAddressTransactionsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetRawAddressTransactionsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetRawAddressTransactionsResponse}
 */
proto.pb.GetRawAddressTransactionsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addConfirmedTransactions(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addUnconfirmedTransactions(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetRawAddressTransactionsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetRawAddressTransactionsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetRawAddressTransactionsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getConfirmedTransactionsList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      1,
      f
    );
  }
  f = message.getUnconfirmedTransactionsList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      2,
      f
    );
  }
};


/**
 * repeated bytes confirmed_transactions = 1;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.getConfirmedTransactionsList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * repeated bytes confirmed_transactions = 1;
 * This is a type-conversion wrapper around `getConfirmedTransactionsList()`
 * @return {!Array<string>}
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.getConfirmedTransactionsList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getConfirmedTransactionsList()));
};


/**
 * repeated bytes confirmed_transactions = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getConfirmedTransactionsList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.getConfirmedTransactionsList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getConfirmedTransactionsList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.GetRawAddressTransactionsResponse.prototype.setConfirmedTransactionsList = function(value) {
  jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.addConfirmedTransactions = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


proto.pb.GetRawAddressTransactionsResponse.prototype.clearConfirmedTransactionsList = function() {
  this.setConfirmedTransactionsList([]);
};


/**
 * repeated bytes unconfirmed_transactions = 2;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.getUnconfirmedTransactionsList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * repeated bytes unconfirmed_transactions = 2;
 * This is a type-conversion wrapper around `getUnconfirmedTransactionsList()`
 * @return {!Array<string>}
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.getUnconfirmedTransactionsList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getUnconfirmedTransactionsList()));
};


/**
 * repeated bytes unconfirmed_transactions = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getUnconfirmedTransactionsList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.getUnconfirmedTransactionsList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getUnconfirmedTransactionsList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.GetRawAddressTransactionsResponse.prototype.setUnconfirmedTransactionsList = function(value) {
  jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.GetRawAddressTransactionsResponse.prototype.addUnconfirmedTransactions = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


proto.pb.GetRawAddressTransactionsResponse.prototype.clearUnconfirmedTransactionsList = function() {
  this.setUnconfirmedTransactionsList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetAddressUnspentOutputsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetAddressUnspentOutputsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetAddressUnspentOutputsRequest.displayName = 'proto.pb.GetAddressUnspentOutputsRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetAddressUnspentOutputsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetAddressUnspentOutputsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetAddressUnspentOutputsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetAddressUnspentOutputsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    address: jspb.Message.getFieldWithDefault(msg, 1, ""),
    includeMempool: jspb.Message.getFieldWithDefault(msg, 2, false),
    includeTokenMetadata: jspb.Message.getFieldWithDefault(msg, 3, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetAddressUnspentOutputsRequest}
 */
proto.pb.GetAddressUnspentOutputsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetAddressUnspentOutputsRequest;
  return proto.pb.GetAddressUnspentOutputsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetAddressUnspentOutputsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetAddressUnspentOutputsRequest}
 */
proto.pb.GetAddressUnspentOutputsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAddress(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeMempool(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeTokenMetadata(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetAddressUnspentOutputsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetAddressUnspentOutputsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetAddressUnspentOutputsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetAddressUnspentOutputsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAddress();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIncludeMempool();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getIncludeTokenMetadata();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional string address = 1;
 * @return {string}
 */
proto.pb.GetAddressUnspentOutputsRequest.prototype.getAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/** @param {string} value */
proto.pb.GetAddressUnspentOutputsRequest.prototype.setAddress = function(value) {
  jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool include_mempool = 2;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetAddressUnspentOutputsRequest.prototype.getIncludeMempool = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 2, false));
};


/** @param {boolean} value */
proto.pb.GetAddressUnspentOutputsRequest.prototype.setIncludeMempool = function(value) {
  jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool include_token_metadata = 3;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetAddressUnspentOutputsRequest.prototype.getIncludeTokenMetadata = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 3, false));
};


/** @param {boolean} value */
proto.pb.GetAddressUnspentOutputsRequest.prototype.setIncludeTokenMetadata = function(value) {
  jspb.Message.setProto3BooleanField(this, 3, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetAddressUnspentOutputsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetAddressUnspentOutputsResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetAddressUnspentOutputsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetAddressUnspentOutputsResponse.displayName = 'proto.pb.GetAddressUnspentOutputsResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetAddressUnspentOutputsResponse.repeatedFields_ = [1,2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetAddressUnspentOutputsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetAddressUnspentOutputsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetAddressUnspentOutputsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetAddressUnspentOutputsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    outputsList: jspb.Message.toObjectList(msg.getOutputsList(),
    proto.pb.UnspentOutput.toObject, includeInstance),
    tokenMetadataList: jspb.Message.toObjectList(msg.getTokenMetadataList(),
    proto.pb.SlpTokenMetadata.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetAddressUnspentOutputsResponse}
 */
proto.pb.GetAddressUnspentOutputsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetAddressUnspentOutputsResponse;
  return proto.pb.GetAddressUnspentOutputsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetAddressUnspentOutputsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetAddressUnspentOutputsResponse}
 */
proto.pb.GetAddressUnspentOutputsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.UnspentOutput;
      reader.readMessage(value,proto.pb.UnspentOutput.deserializeBinaryFromReader);
      msg.addOutputs(value);
      break;
    case 2:
      var value = new proto.pb.SlpTokenMetadata;
      reader.readMessage(value,proto.pb.SlpTokenMetadata.deserializeBinaryFromReader);
      msg.addTokenMetadata(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetAddressUnspentOutputsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetAddressUnspentOutputsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetAddressUnspentOutputsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetAddressUnspentOutputsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOutputsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.pb.UnspentOutput.serializeBinaryToWriter
    );
  }
  f = message.getTokenMetadataList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.pb.SlpTokenMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * repeated UnspentOutput outputs = 1;
 * @return {!Array<!proto.pb.UnspentOutput>}
 */
proto.pb.GetAddressUnspentOutputsResponse.prototype.getOutputsList = function() {
  return /** @type{!Array<!proto.pb.UnspentOutput>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.UnspentOutput, 1));
};


/** @param {!Array<!proto.pb.UnspentOutput>} value */
proto.pb.GetAddressUnspentOutputsResponse.prototype.setOutputsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.pb.UnspentOutput=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.UnspentOutput}
 */
proto.pb.GetAddressUnspentOutputsResponse.prototype.addOutputs = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.pb.UnspentOutput, opt_index);
};


proto.pb.GetAddressUnspentOutputsResponse.prototype.clearOutputsList = function() {
  this.setOutputsList([]);
};


/**
 * repeated SlpTokenMetadata token_metadata = 2;
 * @return {!Array<!proto.pb.SlpTokenMetadata>}
 */
proto.pb.GetAddressUnspentOutputsResponse.prototype.getTokenMetadataList = function() {
  return /** @type{!Array<!proto.pb.SlpTokenMetadata>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.SlpTokenMetadata, 2));
};


/** @param {!Array<!proto.pb.SlpTokenMetadata>} value */
proto.pb.GetAddressUnspentOutputsResponse.prototype.setTokenMetadataList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.pb.SlpTokenMetadata=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.SlpTokenMetadata}
 */
proto.pb.GetAddressUnspentOutputsResponse.prototype.addTokenMetadata = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.pb.SlpTokenMetadata, opt_index);
};


proto.pb.GetAddressUnspentOutputsResponse.prototype.clearTokenMetadataList = function() {
  this.setTokenMetadataList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetUnspentOutputRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetUnspentOutputRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetUnspentOutputRequest.displayName = 'proto.pb.GetUnspentOutputRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetUnspentOutputRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetUnspentOutputRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetUnspentOutputRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetUnspentOutputRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    index: jspb.Message.getFieldWithDefault(msg, 2, 0),
    includeMempool: jspb.Message.getFieldWithDefault(msg, 3, false),
    includeTokenMetadata: jspb.Message.getFieldWithDefault(msg, 4, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetUnspentOutputRequest}
 */
proto.pb.GetUnspentOutputRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetUnspentOutputRequest;
  return proto.pb.GetUnspentOutputRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetUnspentOutputRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetUnspentOutputRequest}
 */
proto.pb.GetUnspentOutputRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setIndex(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeMempool(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeTokenMetadata(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetUnspentOutputRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetUnspentOutputRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetUnspentOutputRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetUnspentOutputRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getIndex();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getIncludeMempool();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getIncludeTokenMetadata();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetUnspentOutputRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetUnspentOutputRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetUnspentOutputRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetUnspentOutputRequest.prototype.setHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional uint32 index = 2;
 * @return {number}
 */
proto.pb.GetUnspentOutputRequest.prototype.getIndex = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetUnspentOutputRequest.prototype.setIndex = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional bool include_mempool = 3;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetUnspentOutputRequest.prototype.getIncludeMempool = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 3, false));
};


/** @param {boolean} value */
proto.pb.GetUnspentOutputRequest.prototype.setIncludeMempool = function(value) {
  jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional bool include_token_metadata = 4;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetUnspentOutputRequest.prototype.getIncludeTokenMetadata = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 4, false));
};


/** @param {boolean} value */
proto.pb.GetUnspentOutputRequest.prototype.setIncludeTokenMetadata = function(value) {
  jspb.Message.setProto3BooleanField(this, 4, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetUnspentOutputResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetUnspentOutputResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetUnspentOutputResponse.displayName = 'proto.pb.GetUnspentOutputResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetUnspentOutputResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetUnspentOutputResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetUnspentOutputResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetUnspentOutputResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    outpoint: (f = msg.getOutpoint()) && proto.pb.Transaction.Input.Outpoint.toObject(includeInstance, f),
    pubkeyScript: msg.getPubkeyScript_asB64(),
    value: jspb.Message.getFieldWithDefault(msg, 3, 0),
    isCoinbase: jspb.Message.getFieldWithDefault(msg, 4, false),
    blockHeight: jspb.Message.getFieldWithDefault(msg, 5, 0),
    slpToken: (f = msg.getSlpToken()) && proto.pb.SlpToken.toObject(includeInstance, f),
    tokenMetadata: (f = msg.getTokenMetadata()) && proto.pb.SlpTokenMetadata.toObject(includeInstance, f),
    cashToken: (f = msg.getCashToken()) && proto.pb.CashToken.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetUnspentOutputResponse}
 */
proto.pb.GetUnspentOutputResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetUnspentOutputResponse;
  return proto.pb.GetUnspentOutputResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetUnspentOutputResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetUnspentOutputResponse}
 */
proto.pb.GetUnspentOutputResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.Transaction.Input.Outpoint;
      reader.readMessage(value,proto.pb.Transaction.Input.Outpoint.deserializeBinaryFromReader);
      msg.setOutpoint(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPubkeyScript(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setValue(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIsCoinbase(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setBlockHeight(value);
      break;
    case 6:
      var value = new proto.pb.SlpToken;
      reader.readMessage(value,proto.pb.SlpToken.deserializeBinaryFromReader);
      msg.setSlpToken(value);
      break;
    case 7:
      var value = new proto.pb.SlpTokenMetadata;
      reader.readMessage(value,proto.pb.SlpTokenMetadata.deserializeBinaryFromReader);
      msg.setTokenMetadata(value);
      break;
    case 8:
      var value = new proto.pb.CashToken;
      reader.readMessage(value,proto.pb.CashToken.deserializeBinaryFromReader);
      msg.setCashToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetUnspentOutputResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetUnspentOutputResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetUnspentOutputResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetUnspentOutputResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOutpoint();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.Transaction.Input.Outpoint.serializeBinaryToWriter
    );
  }
  f = message.getPubkeyScript_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
  f = message.getValue();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
  f = message.getIsCoinbase();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getBlockHeight();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
  f = message.getSlpToken();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      proto.pb.SlpToken.serializeBinaryToWriter
    );
  }
  f = message.getTokenMetadata();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.pb.SlpTokenMetadata.serializeBinaryToWriter
    );
  }
  f = message.getCashToken();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      proto.pb.CashToken.serializeBinaryToWriter
    );
  }
};


/**
 * optional Transaction.Input.Outpoint outpoint = 1;
 * @return {?proto.pb.Transaction.Input.Outpoint}
 */
proto.pb.GetUnspentOutputResponse.prototype.getOutpoint = function() {
  return /** @type{?proto.pb.Transaction.Input.Outpoint} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction.Input.Outpoint, 1));
};


/** @param {?proto.pb.Transaction.Input.Outpoint|undefined} value */
proto.pb.GetUnspentOutputResponse.prototype.setOutpoint = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.GetUnspentOutputResponse.prototype.clearOutpoint = function() {
  this.setOutpoint(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetUnspentOutputResponse.prototype.hasOutpoint = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bytes pubkey_script = 2;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetUnspentOutputResponse.prototype.getPubkeyScript = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes pubkey_script = 2;
 * This is a type-conversion wrapper around `getPubkeyScript()`
 * @return {string}
 */
proto.pb.GetUnspentOutputResponse.prototype.getPubkeyScript_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPubkeyScript()));
};


/**
 * optional bytes pubkey_script = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPubkeyScript()`
 * @return {!Uint8Array}
 */
proto.pb.GetUnspentOutputResponse.prototype.getPubkeyScript_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPubkeyScript()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetUnspentOutputResponse.prototype.setPubkeyScript = function(value) {
  jspb.Message.setProto3BytesField(this, 2, value);
};


/**
 * optional int64 value = 3;
 * @return {number}
 */
proto.pb.GetUnspentOutputResponse.prototype.getValue = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {number} value */
proto.pb.GetUnspentOutputResponse.prototype.setValue = function(value) {
  jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional bool is_coinbase = 4;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetUnspentOutputResponse.prototype.getIsCoinbase = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 4, false));
};


/** @param {boolean} value */
proto.pb.GetUnspentOutputResponse.prototype.setIsCoinbase = function(value) {
  jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional int32 block_height = 5;
 * @return {number}
 */
proto.pb.GetUnspentOutputResponse.prototype.getBlockHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.GetUnspentOutputResponse.prototype.setBlockHeight = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional SlpToken slp_token = 6;
 * @return {?proto.pb.SlpToken}
 */
proto.pb.GetUnspentOutputResponse.prototype.getSlpToken = function() {
  return /** @type{?proto.pb.SlpToken} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpToken, 6));
};


/** @param {?proto.pb.SlpToken|undefined} value */
proto.pb.GetUnspentOutputResponse.prototype.setSlpToken = function(value) {
  jspb.Message.setWrapperField(this, 6, value);
};


proto.pb.GetUnspentOutputResponse.prototype.clearSlpToken = function() {
  this.setSlpToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetUnspentOutputResponse.prototype.hasSlpToken = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional SlpTokenMetadata token_metadata = 7;
 * @return {?proto.pb.SlpTokenMetadata}
 */
proto.pb.GetUnspentOutputResponse.prototype.getTokenMetadata = function() {
  return /** @type{?proto.pb.SlpTokenMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpTokenMetadata, 7));
};


/** @param {?proto.pb.SlpTokenMetadata|undefined} value */
proto.pb.GetUnspentOutputResponse.prototype.setTokenMetadata = function(value) {
  jspb.Message.setWrapperField(this, 7, value);
};


proto.pb.GetUnspentOutputResponse.prototype.clearTokenMetadata = function() {
  this.setTokenMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetUnspentOutputResponse.prototype.hasTokenMetadata = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * optional CashToken cash_token = 8;
 * @return {?proto.pb.CashToken}
 */
proto.pb.GetUnspentOutputResponse.prototype.getCashToken = function() {
  return /** @type{?proto.pb.CashToken} */ (
    jspb.Message.getWrapperField(this, proto.pb.CashToken, 8));
};


/** @param {?proto.pb.CashToken|undefined} value */
proto.pb.GetUnspentOutputResponse.prototype.setCashToken = function(value) {
  jspb.Message.setWrapperField(this, 8, value);
};


proto.pb.GetUnspentOutputResponse.prototype.clearCashToken = function() {
  this.setCashToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetUnspentOutputResponse.prototype.hasCashToken = function() {
  return jspb.Message.getField(this, 8) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetMerkleProofRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetMerkleProofRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetMerkleProofRequest.displayName = 'proto.pb.GetMerkleProofRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetMerkleProofRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetMerkleProofRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetMerkleProofRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMerkleProofRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    transactionHash: msg.getTransactionHash_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetMerkleProofRequest}
 */
proto.pb.GetMerkleProofRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetMerkleProofRequest;
  return proto.pb.GetMerkleProofRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetMerkleProofRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetMerkleProofRequest}
 */
proto.pb.GetMerkleProofRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTransactionHash(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetMerkleProofRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetMerkleProofRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetMerkleProofRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMerkleProofRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTransactionHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes transaction_hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetMerkleProofRequest.prototype.getTransactionHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes transaction_hash = 1;
 * This is a type-conversion wrapper around `getTransactionHash()`
 * @return {string}
 */
proto.pb.GetMerkleProofRequest.prototype.getTransactionHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTransactionHash()));
};


/**
 * optional bytes transaction_hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTransactionHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetMerkleProofRequest.prototype.getTransactionHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTransactionHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetMerkleProofRequest.prototype.setTransactionHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetMerkleProofResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetMerkleProofResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetMerkleProofResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetMerkleProofResponse.displayName = 'proto.pb.GetMerkleProofResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetMerkleProofResponse.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetMerkleProofResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetMerkleProofResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetMerkleProofResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMerkleProofResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    block: (f = msg.getBlock()) && proto.pb.BlockInfo.toObject(includeInstance, f),
    hashesList: msg.getHashesList_asB64(),
    flags: msg.getFlags_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetMerkleProofResponse}
 */
proto.pb.GetMerkleProofResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetMerkleProofResponse;
  return proto.pb.GetMerkleProofResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetMerkleProofResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetMerkleProofResponse}
 */
proto.pb.GetMerkleProofResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.BlockInfo;
      reader.readMessage(value,proto.pb.BlockInfo.deserializeBinaryFromReader);
      msg.setBlock(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addHashes(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setFlags(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetMerkleProofResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetMerkleProofResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetMerkleProofResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetMerkleProofResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBlock();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.BlockInfo.serializeBinaryToWriter
    );
  }
  f = message.getHashesList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      2,
      f
    );
  }
  f = message.getFlags_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
};


/**
 * optional BlockInfo block = 1;
 * @return {?proto.pb.BlockInfo}
 */
proto.pb.GetMerkleProofResponse.prototype.getBlock = function() {
  return /** @type{?proto.pb.BlockInfo} */ (
    jspb.Message.getWrapperField(this, proto.pb.BlockInfo, 1));
};


/** @param {?proto.pb.BlockInfo|undefined} value */
proto.pb.GetMerkleProofResponse.prototype.setBlock = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.GetMerkleProofResponse.prototype.clearBlock = function() {
  this.setBlock(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetMerkleProofResponse.prototype.hasBlock = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * repeated bytes hashes = 2;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.GetMerkleProofResponse.prototype.getHashesList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * repeated bytes hashes = 2;
 * This is a type-conversion wrapper around `getHashesList()`
 * @return {!Array<string>}
 */
proto.pb.GetMerkleProofResponse.prototype.getHashesList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getHashesList()));
};


/**
 * repeated bytes hashes = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHashesList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.GetMerkleProofResponse.prototype.getHashesList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getHashesList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.GetMerkleProofResponse.prototype.setHashesList = function(value) {
  jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.GetMerkleProofResponse.prototype.addHashes = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


proto.pb.GetMerkleProofResponse.prototype.clearHashesList = function() {
  this.setHashesList([]);
};


/**
 * optional bytes flags = 3;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetMerkleProofResponse.prototype.getFlags = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes flags = 3;
 * This is a type-conversion wrapper around `getFlags()`
 * @return {string}
 */
proto.pb.GetMerkleProofResponse.prototype.getFlags_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getFlags()));
};


/**
 * optional bytes flags = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getFlags()`
 * @return {!Uint8Array}
 */
proto.pb.GetMerkleProofResponse.prototype.getFlags_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getFlags()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetMerkleProofResponse.prototype.setFlags = function(value) {
  jspb.Message.setProto3BytesField(this, 3, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SubmitTransactionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.SubmitTransactionRequest.repeatedFields_, null);
};
goog.inherits(proto.pb.SubmitTransactionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SubmitTransactionRequest.displayName = 'proto.pb.SubmitTransactionRequest';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.SubmitTransactionRequest.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SubmitTransactionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SubmitTransactionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SubmitTransactionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SubmitTransactionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    transaction: msg.getTransaction_asB64(),
    skipSlpValidityCheck: jspb.Message.getFieldWithDefault(msg, 2, false),
    requiredSlpBurnsList: jspb.Message.toObjectList(msg.getRequiredSlpBurnsList(),
    proto.pb.SlpRequiredBurn.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SubmitTransactionRequest}
 */
proto.pb.SubmitTransactionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SubmitTransactionRequest;
  return proto.pb.SubmitTransactionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SubmitTransactionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SubmitTransactionRequest}
 */
proto.pb.SubmitTransactionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTransaction(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSkipSlpValidityCheck(value);
      break;
    case 3:
      var value = new proto.pb.SlpRequiredBurn;
      reader.readMessage(value,proto.pb.SlpRequiredBurn.deserializeBinaryFromReader);
      msg.addRequiredSlpBurns(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SubmitTransactionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SubmitTransactionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SubmitTransactionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SubmitTransactionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTransaction_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getSkipSlpValidityCheck();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getRequiredSlpBurnsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.pb.SlpRequiredBurn.serializeBinaryToWriter
    );
  }
};


/**
 * optional bytes transaction = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SubmitTransactionRequest.prototype.getTransaction = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes transaction = 1;
 * This is a type-conversion wrapper around `getTransaction()`
 * @return {string}
 */
proto.pb.SubmitTransactionRequest.prototype.getTransaction_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTransaction()));
};


/**
 * optional bytes transaction = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTransaction()`
 * @return {!Uint8Array}
 */
proto.pb.SubmitTransactionRequest.prototype.getTransaction_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTransaction()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SubmitTransactionRequest.prototype.setTransaction = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional bool skip_slp_validity_check = 2;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.SubmitTransactionRequest.prototype.getSkipSlpValidityCheck = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 2, false));
};


/** @param {boolean} value */
proto.pb.SubmitTransactionRequest.prototype.setSkipSlpValidityCheck = function(value) {
  jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * repeated SlpRequiredBurn required_slp_burns = 3;
 * @return {!Array<!proto.pb.SlpRequiredBurn>}
 */
proto.pb.SubmitTransactionRequest.prototype.getRequiredSlpBurnsList = function() {
  return /** @type{!Array<!proto.pb.SlpRequiredBurn>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.SlpRequiredBurn, 3));
};


/** @param {!Array<!proto.pb.SlpRequiredBurn>} value */
proto.pb.SubmitTransactionRequest.prototype.setRequiredSlpBurnsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.pb.SlpRequiredBurn=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.SlpRequiredBurn}
 */
proto.pb.SubmitTransactionRequest.prototype.addRequiredSlpBurns = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.pb.SlpRequiredBurn, opt_index);
};


proto.pb.SubmitTransactionRequest.prototype.clearRequiredSlpBurnsList = function() {
  this.setRequiredSlpBurnsList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SubmitTransactionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SubmitTransactionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SubmitTransactionResponse.displayName = 'proto.pb.SubmitTransactionResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SubmitTransactionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SubmitTransactionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SubmitTransactionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SubmitTransactionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SubmitTransactionResponse}
 */
proto.pb.SubmitTransactionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SubmitTransactionResponse;
  return proto.pb.SubmitTransactionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SubmitTransactionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SubmitTransactionResponse}
 */
proto.pb.SubmitTransactionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SubmitTransactionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SubmitTransactionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SubmitTransactionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SubmitTransactionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SubmitTransactionResponse.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.SubmitTransactionResponse.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.SubmitTransactionResponse.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SubmitTransactionResponse.prototype.setHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.CheckSlpTransactionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.CheckSlpTransactionRequest.repeatedFields_, null);
};
goog.inherits(proto.pb.CheckSlpTransactionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.CheckSlpTransactionRequest.displayName = 'proto.pb.CheckSlpTransactionRequest';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.CheckSlpTransactionRequest.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.CheckSlpTransactionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.CheckSlpTransactionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.CheckSlpTransactionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.CheckSlpTransactionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    transaction: msg.getTransaction_asB64(),
    requiredSlpBurnsList: jspb.Message.toObjectList(msg.getRequiredSlpBurnsList(),
    proto.pb.SlpRequiredBurn.toObject, includeInstance),
    useSpecValidityJudgement: jspb.Message.getFieldWithDefault(msg, 3, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.CheckSlpTransactionRequest}
 */
proto.pb.CheckSlpTransactionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.CheckSlpTransactionRequest;
  return proto.pb.CheckSlpTransactionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.CheckSlpTransactionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.CheckSlpTransactionRequest}
 */
proto.pb.CheckSlpTransactionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTransaction(value);
      break;
    case 2:
      var value = new proto.pb.SlpRequiredBurn;
      reader.readMessage(value,proto.pb.SlpRequiredBurn.deserializeBinaryFromReader);
      msg.addRequiredSlpBurns(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setUseSpecValidityJudgement(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.CheckSlpTransactionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.CheckSlpTransactionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.CheckSlpTransactionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.CheckSlpTransactionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTransaction_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getRequiredSlpBurnsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.pb.SlpRequiredBurn.serializeBinaryToWriter
    );
  }
  f = message.getUseSpecValidityJudgement();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional bytes transaction = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.CheckSlpTransactionRequest.prototype.getTransaction = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes transaction = 1;
 * This is a type-conversion wrapper around `getTransaction()`
 * @return {string}
 */
proto.pb.CheckSlpTransactionRequest.prototype.getTransaction_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTransaction()));
};


/**
 * optional bytes transaction = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTransaction()`
 * @return {!Uint8Array}
 */
proto.pb.CheckSlpTransactionRequest.prototype.getTransaction_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTransaction()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.CheckSlpTransactionRequest.prototype.setTransaction = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * repeated SlpRequiredBurn required_slp_burns = 2;
 * @return {!Array<!proto.pb.SlpRequiredBurn>}
 */
proto.pb.CheckSlpTransactionRequest.prototype.getRequiredSlpBurnsList = function() {
  return /** @type{!Array<!proto.pb.SlpRequiredBurn>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.SlpRequiredBurn, 2));
};


/** @param {!Array<!proto.pb.SlpRequiredBurn>} value */
proto.pb.CheckSlpTransactionRequest.prototype.setRequiredSlpBurnsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.pb.SlpRequiredBurn=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.SlpRequiredBurn}
 */
proto.pb.CheckSlpTransactionRequest.prototype.addRequiredSlpBurns = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.pb.SlpRequiredBurn, opt_index);
};


proto.pb.CheckSlpTransactionRequest.prototype.clearRequiredSlpBurnsList = function() {
  this.setRequiredSlpBurnsList([]);
};


/**
 * optional bool use_spec_validity_judgement = 3;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.CheckSlpTransactionRequest.prototype.getUseSpecValidityJudgement = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 3, false));
};


/** @param {boolean} value */
proto.pb.CheckSlpTransactionRequest.prototype.setUseSpecValidityJudgement = function(value) {
  jspb.Message.setProto3BooleanField(this, 3, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.CheckSlpTransactionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.CheckSlpTransactionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.CheckSlpTransactionResponse.displayName = 'proto.pb.CheckSlpTransactionResponse';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.CheckSlpTransactionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.CheckSlpTransactionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.CheckSlpTransactionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.CheckSlpTransactionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    isValid: jspb.Message.getFieldWithDefault(msg, 1, false),
    invalidReason: jspb.Message.getFieldWithDefault(msg, 2, ""),
    bestHeight: jspb.Message.getFieldWithDefault(msg, 3, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.CheckSlpTransactionResponse}
 */
proto.pb.CheckSlpTransactionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.CheckSlpTransactionResponse;
  return proto.pb.CheckSlpTransactionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.CheckSlpTransactionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.CheckSlpTransactionResponse}
 */
proto.pb.CheckSlpTransactionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIsValid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setInvalidReason(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setBestHeight(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.CheckSlpTransactionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.CheckSlpTransactionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.CheckSlpTransactionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.CheckSlpTransactionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIsValid();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getInvalidReason();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getBestHeight();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
};


/**
 * optional bool is_valid = 1;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.CheckSlpTransactionResponse.prototype.getIsValid = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 1, false));
};


/** @param {boolean} value */
proto.pb.CheckSlpTransactionResponse.prototype.setIsValid = function(value) {
  jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string invalid_reason = 2;
 * @return {string}
 */
proto.pb.CheckSlpTransactionResponse.prototype.getInvalidReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/** @param {string} value */
proto.pb.CheckSlpTransactionResponse.prototype.setInvalidReason = function(value) {
  jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 best_height = 3;
 * @return {number}
 */
proto.pb.CheckSlpTransactionResponse.prototype.getBestHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {number} value */
proto.pb.CheckSlpTransactionResponse.prototype.setBestHeight = function(value) {
  jspb.Message.setProto3IntField(this, 3, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SubscribeTransactionsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SubscribeTransactionsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SubscribeTransactionsRequest.displayName = 'proto.pb.SubscribeTransactionsRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SubscribeTransactionsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SubscribeTransactionsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SubscribeTransactionsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SubscribeTransactionsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    subscribe: (f = msg.getSubscribe()) && proto.pb.TransactionFilter.toObject(includeInstance, f),
    unsubscribe: (f = msg.getUnsubscribe()) && proto.pb.TransactionFilter.toObject(includeInstance, f),
    includeMempool: jspb.Message.getFieldWithDefault(msg, 3, false),
    includeInBlock: jspb.Message.getFieldWithDefault(msg, 4, false),
    serializeTx: jspb.Message.getFieldWithDefault(msg, 5, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SubscribeTransactionsRequest}
 */
proto.pb.SubscribeTransactionsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SubscribeTransactionsRequest;
  return proto.pb.SubscribeTransactionsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SubscribeTransactionsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SubscribeTransactionsRequest}
 */
proto.pb.SubscribeTransactionsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.TransactionFilter;
      reader.readMessage(value,proto.pb.TransactionFilter.deserializeBinaryFromReader);
      msg.setSubscribe(value);
      break;
    case 2:
      var value = new proto.pb.TransactionFilter;
      reader.readMessage(value,proto.pb.TransactionFilter.deserializeBinaryFromReader);
      msg.setUnsubscribe(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeMempool(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeInBlock(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSerializeTx(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SubscribeTransactionsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SubscribeTransactionsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SubscribeTransactionsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SubscribeTransactionsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSubscribe();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.TransactionFilter.serializeBinaryToWriter
    );
  }
  f = message.getUnsubscribe();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.pb.TransactionFilter.serializeBinaryToWriter
    );
  }
  f = message.getIncludeMempool();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getIncludeInBlock();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getSerializeTx();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
};


/**
 * optional TransactionFilter subscribe = 1;
 * @return {?proto.pb.TransactionFilter}
 */
proto.pb.SubscribeTransactionsRequest.prototype.getSubscribe = function() {
  return /** @type{?proto.pb.TransactionFilter} */ (
    jspb.Message.getWrapperField(this, proto.pb.TransactionFilter, 1));
};


/** @param {?proto.pb.TransactionFilter|undefined} value */
proto.pb.SubscribeTransactionsRequest.prototype.setSubscribe = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.SubscribeTransactionsRequest.prototype.clearSubscribe = function() {
  this.setSubscribe(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SubscribeTransactionsRequest.prototype.hasSubscribe = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional TransactionFilter unsubscribe = 2;
 * @return {?proto.pb.TransactionFilter}
 */
proto.pb.SubscribeTransactionsRequest.prototype.getUnsubscribe = function() {
  return /** @type{?proto.pb.TransactionFilter} */ (
    jspb.Message.getWrapperField(this, proto.pb.TransactionFilter, 2));
};


/** @param {?proto.pb.TransactionFilter|undefined} value */
proto.pb.SubscribeTransactionsRequest.prototype.setUnsubscribe = function(value) {
  jspb.Message.setWrapperField(this, 2, value);
};


proto.pb.SubscribeTransactionsRequest.prototype.clearUnsubscribe = function() {
  this.setUnsubscribe(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SubscribeTransactionsRequest.prototype.hasUnsubscribe = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool include_mempool = 3;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.SubscribeTransactionsRequest.prototype.getIncludeMempool = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 3, false));
};


/** @param {boolean} value */
proto.pb.SubscribeTransactionsRequest.prototype.setIncludeMempool = function(value) {
  jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional bool include_in_block = 4;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.SubscribeTransactionsRequest.prototype.getIncludeInBlock = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 4, false));
};


/** @param {boolean} value */
proto.pb.SubscribeTransactionsRequest.prototype.setIncludeInBlock = function(value) {
  jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional bool serialize_tx = 5;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.SubscribeTransactionsRequest.prototype.getSerializeTx = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 5, false));
};


/** @param {boolean} value */
proto.pb.SubscribeTransactionsRequest.prototype.setSerializeTx = function(value) {
  jspb.Message.setProto3BooleanField(this, 5, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SubscribeBlocksRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SubscribeBlocksRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SubscribeBlocksRequest.displayName = 'proto.pb.SubscribeBlocksRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SubscribeBlocksRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SubscribeBlocksRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SubscribeBlocksRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SubscribeBlocksRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    fullBlock: jspb.Message.getFieldWithDefault(msg, 1, false),
    fullTransactions: jspb.Message.getFieldWithDefault(msg, 2, false),
    serializeBlock: jspb.Message.getFieldWithDefault(msg, 3, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SubscribeBlocksRequest}
 */
proto.pb.SubscribeBlocksRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SubscribeBlocksRequest;
  return proto.pb.SubscribeBlocksRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SubscribeBlocksRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SubscribeBlocksRequest}
 */
proto.pb.SubscribeBlocksRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setFullBlock(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setFullTransactions(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSerializeBlock(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SubscribeBlocksRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SubscribeBlocksRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SubscribeBlocksRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SubscribeBlocksRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFullBlock();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getFullTransactions();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getSerializeBlock();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional bool full_block = 1;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.SubscribeBlocksRequest.prototype.getFullBlock = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 1, false));
};


/** @param {boolean} value */
proto.pb.SubscribeBlocksRequest.prototype.setFullBlock = function(value) {
  jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional bool full_transactions = 2;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.SubscribeBlocksRequest.prototype.getFullTransactions = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 2, false));
};


/** @param {boolean} value */
proto.pb.SubscribeBlocksRequest.prototype.setFullTransactions = function(value) {
  jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool serialize_block = 3;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.SubscribeBlocksRequest.prototype.getSerializeBlock = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 3, false));
};


/** @param {boolean} value */
proto.pb.SubscribeBlocksRequest.prototype.setSerializeBlock = function(value) {
  jspb.Message.setProto3BooleanField(this, 3, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpTokenMetadataRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetSlpTokenMetadataRequest.repeatedFields_, null);
};
goog.inherits(proto.pb.GetSlpTokenMetadataRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpTokenMetadataRequest.displayName = 'proto.pb.GetSlpTokenMetadataRequest';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetSlpTokenMetadataRequest.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpTokenMetadataRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpTokenMetadataRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpTokenMetadataRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTokenMetadataRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    tokenIdsList: msg.getTokenIdsList_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpTokenMetadataRequest}
 */
proto.pb.GetSlpTokenMetadataRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpTokenMetadataRequest;
  return proto.pb.GetSlpTokenMetadataRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpTokenMetadataRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpTokenMetadataRequest}
 */
proto.pb.GetSlpTokenMetadataRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addTokenIds(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTokenMetadataRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpTokenMetadataRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpTokenMetadataRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTokenMetadataRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTokenIdsList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      1,
      f
    );
  }
};


/**
 * repeated bytes token_ids = 1;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.GetSlpTokenMetadataRequest.prototype.getTokenIdsList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * repeated bytes token_ids = 1;
 * This is a type-conversion wrapper around `getTokenIdsList()`
 * @return {!Array<string>}
 */
proto.pb.GetSlpTokenMetadataRequest.prototype.getTokenIdsList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getTokenIdsList()));
};


/**
 * repeated bytes token_ids = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenIdsList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.GetSlpTokenMetadataRequest.prototype.getTokenIdsList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getTokenIdsList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.GetSlpTokenMetadataRequest.prototype.setTokenIdsList = function(value) {
  jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.GetSlpTokenMetadataRequest.prototype.addTokenIds = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


proto.pb.GetSlpTokenMetadataRequest.prototype.clearTokenIdsList = function() {
  this.setTokenIdsList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpTokenMetadataResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetSlpTokenMetadataResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetSlpTokenMetadataResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpTokenMetadataResponse.displayName = 'proto.pb.GetSlpTokenMetadataResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetSlpTokenMetadataResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpTokenMetadataResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpTokenMetadataResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpTokenMetadataResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTokenMetadataResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    tokenMetadataList: jspb.Message.toObjectList(msg.getTokenMetadataList(),
    proto.pb.SlpTokenMetadata.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpTokenMetadataResponse}
 */
proto.pb.GetSlpTokenMetadataResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpTokenMetadataResponse;
  return proto.pb.GetSlpTokenMetadataResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpTokenMetadataResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpTokenMetadataResponse}
 */
proto.pb.GetSlpTokenMetadataResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.SlpTokenMetadata;
      reader.readMessage(value,proto.pb.SlpTokenMetadata.deserializeBinaryFromReader);
      msg.addTokenMetadata(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTokenMetadataResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpTokenMetadataResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpTokenMetadataResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTokenMetadataResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTokenMetadataList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.pb.SlpTokenMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * repeated SlpTokenMetadata token_metadata = 1;
 * @return {!Array<!proto.pb.SlpTokenMetadata>}
 */
proto.pb.GetSlpTokenMetadataResponse.prototype.getTokenMetadataList = function() {
  return /** @type{!Array<!proto.pb.SlpTokenMetadata>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.SlpTokenMetadata, 1));
};


/** @param {!Array<!proto.pb.SlpTokenMetadata>} value */
proto.pb.GetSlpTokenMetadataResponse.prototype.setTokenMetadataList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.pb.SlpTokenMetadata=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.SlpTokenMetadata}
 */
proto.pb.GetSlpTokenMetadataResponse.prototype.addTokenMetadata = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.pb.SlpTokenMetadata, opt_index);
};


proto.pb.GetSlpTokenMetadataResponse.prototype.clearTokenMetadataList = function() {
  this.setTokenMetadataList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpParsedScriptRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.GetSlpParsedScriptRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpParsedScriptRequest.displayName = 'proto.pb.GetSlpParsedScriptRequest';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpParsedScriptRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpParsedScriptRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpParsedScriptRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpParsedScriptRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    slpOpreturnScript: msg.getSlpOpreturnScript_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpParsedScriptRequest}
 */
proto.pb.GetSlpParsedScriptRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpParsedScriptRequest;
  return proto.pb.GetSlpParsedScriptRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpParsedScriptRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpParsedScriptRequest}
 */
proto.pb.GetSlpParsedScriptRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSlpOpreturnScript(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpParsedScriptRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpParsedScriptRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpParsedScriptRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpParsedScriptRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSlpOpreturnScript_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes slp_opreturn_script = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetSlpParsedScriptRequest.prototype.getSlpOpreturnScript = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes slp_opreturn_script = 1;
 * This is a type-conversion wrapper around `getSlpOpreturnScript()`
 * @return {string}
 */
proto.pb.GetSlpParsedScriptRequest.prototype.getSlpOpreturnScript_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSlpOpreturnScript()));
};


/**
 * optional bytes slp_opreturn_script = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSlpOpreturnScript()`
 * @return {!Uint8Array}
 */
proto.pb.GetSlpParsedScriptRequest.prototype.getSlpOpreturnScript_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSlpOpreturnScript()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetSlpParsedScriptRequest.prototype.setSlpOpreturnScript = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpParsedScriptResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetSlpParsedScriptResponse.oneofGroups_);
};
goog.inherits(proto.pb.GetSlpParsedScriptResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpParsedScriptResponse.displayName = 'proto.pb.GetSlpParsedScriptResponse';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetSlpParsedScriptResponse.oneofGroups_ = [[5,6,7,8,9]];

/**
 * @enum {number}
 */
proto.pb.GetSlpParsedScriptResponse.SlpMetadataCase = {
  SLP_METADATA_NOT_SET: 0,
  V1_GENESIS: 5,
  V1_MINT: 6,
  V1_SEND: 7,
  V1_NFT1_CHILD_GENESIS: 8,
  V1_NFT1_CHILD_SEND: 9
};

/**
 * @return {proto.pb.GetSlpParsedScriptResponse.SlpMetadataCase}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getSlpMetadataCase = function() {
  return /** @type {proto.pb.GetSlpParsedScriptResponse.SlpMetadataCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetSlpParsedScriptResponse.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpParsedScriptResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpParsedScriptResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpParsedScriptResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    parsingError: jspb.Message.getFieldWithDefault(msg, 1, ""),
    tokenId: msg.getTokenId_asB64(),
    slpAction: jspb.Message.getFieldWithDefault(msg, 3, 0),
    tokenType: jspb.Message.getFieldWithDefault(msg, 4, 0),
    v1Genesis: (f = msg.getV1Genesis()) && proto.pb.SlpV1GenesisMetadata.toObject(includeInstance, f),
    v1Mint: (f = msg.getV1Mint()) && proto.pb.SlpV1MintMetadata.toObject(includeInstance, f),
    v1Send: (f = msg.getV1Send()) && proto.pb.SlpV1SendMetadata.toObject(includeInstance, f),
    v1Nft1ChildGenesis: (f = msg.getV1Nft1ChildGenesis()) && proto.pb.SlpV1Nft1ChildGenesisMetadata.toObject(includeInstance, f),
    v1Nft1ChildSend: (f = msg.getV1Nft1ChildSend()) && proto.pb.SlpV1Nft1ChildSendMetadata.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpParsedScriptResponse}
 */
proto.pb.GetSlpParsedScriptResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpParsedScriptResponse;
  return proto.pb.GetSlpParsedScriptResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpParsedScriptResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpParsedScriptResponse}
 */
proto.pb.GetSlpParsedScriptResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setParsingError(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenId(value);
      break;
    case 3:
      var value = /** @type {!proto.pb.SlpAction} */ (reader.readEnum());
      msg.setSlpAction(value);
      break;
    case 4:
      var value = /** @type {!proto.pb.SlpTokenType} */ (reader.readEnum());
      msg.setTokenType(value);
      break;
    case 5:
      var value = new proto.pb.SlpV1GenesisMetadata;
      reader.readMessage(value,proto.pb.SlpV1GenesisMetadata.deserializeBinaryFromReader);
      msg.setV1Genesis(value);
      break;
    case 6:
      var value = new proto.pb.SlpV1MintMetadata;
      reader.readMessage(value,proto.pb.SlpV1MintMetadata.deserializeBinaryFromReader);
      msg.setV1Mint(value);
      break;
    case 7:
      var value = new proto.pb.SlpV1SendMetadata;
      reader.readMessage(value,proto.pb.SlpV1SendMetadata.deserializeBinaryFromReader);
      msg.setV1Send(value);
      break;
    case 8:
      var value = new proto.pb.SlpV1Nft1ChildGenesisMetadata;
      reader.readMessage(value,proto.pb.SlpV1Nft1ChildGenesisMetadata.deserializeBinaryFromReader);
      msg.setV1Nft1ChildGenesis(value);
      break;
    case 9:
      var value = new proto.pb.SlpV1Nft1ChildSendMetadata;
      reader.readMessage(value,proto.pb.SlpV1Nft1ChildSendMetadata.deserializeBinaryFromReader);
      msg.setV1Nft1ChildSend(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpParsedScriptResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpParsedScriptResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpParsedScriptResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParsingError();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTokenId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
  f = message.getSlpAction();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getTokenType();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getV1Genesis();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.pb.SlpV1GenesisMetadata.serializeBinaryToWriter
    );
  }
  f = message.getV1Mint();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      proto.pb.SlpV1MintMetadata.serializeBinaryToWriter
    );
  }
  f = message.getV1Send();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.pb.SlpV1SendMetadata.serializeBinaryToWriter
    );
  }
  f = message.getV1Nft1ChildGenesis();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      proto.pb.SlpV1Nft1ChildGenesisMetadata.serializeBinaryToWriter
    );
  }
  f = message.getV1Nft1ChildSend();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      proto.pb.SlpV1Nft1ChildSendMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional string parsing_error = 1;
 * @return {string}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getParsingError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/** @param {string} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setParsingError = function(value) {
  jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bytes token_id = 2;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getTokenId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes token_id = 2;
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {string}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getTokenId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenId()));
};


/**
 * optional bytes token_id = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {!Uint8Array}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getTokenId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setTokenId = function(value) {
  jspb.Message.setProto3BytesField(this, 2, value);
};


/**
 * optional SlpAction slp_action = 3;
 * @return {!proto.pb.SlpAction}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getSlpAction = function() {
  return /** @type {!proto.pb.SlpAction} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {!proto.pb.SlpAction} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setSlpAction = function(value) {
  jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional SlpTokenType token_type = 4;
 * @return {!proto.pb.SlpTokenType}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getTokenType = function() {
  return /** @type {!proto.pb.SlpTokenType} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/** @param {!proto.pb.SlpTokenType} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setTokenType = function(value) {
  jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional SlpV1GenesisMetadata v1_genesis = 5;
 * @return {?proto.pb.SlpV1GenesisMetadata}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getV1Genesis = function() {
  return /** @type{?proto.pb.SlpV1GenesisMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1GenesisMetadata, 5));
};


/** @param {?proto.pb.SlpV1GenesisMetadata|undefined} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setV1Genesis = function(value) {
  jspb.Message.setOneofWrapperField(this, 5, proto.pb.GetSlpParsedScriptResponse.oneofGroups_[0], value);
};


proto.pb.GetSlpParsedScriptResponse.prototype.clearV1Genesis = function() {
  this.setV1Genesis(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.hasV1Genesis = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional SlpV1MintMetadata v1_mint = 6;
 * @return {?proto.pb.SlpV1MintMetadata}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getV1Mint = function() {
  return /** @type{?proto.pb.SlpV1MintMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1MintMetadata, 6));
};


/** @param {?proto.pb.SlpV1MintMetadata|undefined} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setV1Mint = function(value) {
  jspb.Message.setOneofWrapperField(this, 6, proto.pb.GetSlpParsedScriptResponse.oneofGroups_[0], value);
};


proto.pb.GetSlpParsedScriptResponse.prototype.clearV1Mint = function() {
  this.setV1Mint(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.hasV1Mint = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional SlpV1SendMetadata v1_send = 7;
 * @return {?proto.pb.SlpV1SendMetadata}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getV1Send = function() {
  return /** @type{?proto.pb.SlpV1SendMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1SendMetadata, 7));
};


/** @param {?proto.pb.SlpV1SendMetadata|undefined} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setV1Send = function(value) {
  jspb.Message.setOneofWrapperField(this, 7, proto.pb.GetSlpParsedScriptResponse.oneofGroups_[0], value);
};


proto.pb.GetSlpParsedScriptResponse.prototype.clearV1Send = function() {
  this.setV1Send(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.hasV1Send = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * optional SlpV1Nft1ChildGenesisMetadata v1_nft1_child_genesis = 8;
 * @return {?proto.pb.SlpV1Nft1ChildGenesisMetadata}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getV1Nft1ChildGenesis = function() {
  return /** @type{?proto.pb.SlpV1Nft1ChildGenesisMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1Nft1ChildGenesisMetadata, 8));
};


/** @param {?proto.pb.SlpV1Nft1ChildGenesisMetadata|undefined} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setV1Nft1ChildGenesis = function(value) {
  jspb.Message.setOneofWrapperField(this, 8, proto.pb.GetSlpParsedScriptResponse.oneofGroups_[0], value);
};


proto.pb.GetSlpParsedScriptResponse.prototype.clearV1Nft1ChildGenesis = function() {
  this.setV1Nft1ChildGenesis(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.hasV1Nft1ChildGenesis = function() {
  return jspb.Message.getField(this, 8) != null;
};


/**
 * optional SlpV1Nft1ChildSendMetadata v1_nft1_child_send = 9;
 * @return {?proto.pb.SlpV1Nft1ChildSendMetadata}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.getV1Nft1ChildSend = function() {
  return /** @type{?proto.pb.SlpV1Nft1ChildSendMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1Nft1ChildSendMetadata, 9));
};


/** @param {?proto.pb.SlpV1Nft1ChildSendMetadata|undefined} value */
proto.pb.GetSlpParsedScriptResponse.prototype.setV1Nft1ChildSend = function(value) {
  jspb.Message.setOneofWrapperField(this, 9, proto.pb.GetSlpParsedScriptResponse.oneofGroups_[0], value);
};


proto.pb.GetSlpParsedScriptResponse.prototype.clearV1Nft1ChildSend = function() {
  this.setV1Nft1ChildSend(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetSlpParsedScriptResponse.prototype.hasV1Nft1ChildSend = function() {
  return jspb.Message.getField(this, 9) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpTrustedValidationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetSlpTrustedValidationRequest.repeatedFields_, null);
};
goog.inherits(proto.pb.GetSlpTrustedValidationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpTrustedValidationRequest.displayName = 'proto.pb.GetSlpTrustedValidationRequest';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetSlpTrustedValidationRequest.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpTrustedValidationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpTrustedValidationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpTrustedValidationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTrustedValidationRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    queriesList: jspb.Message.toObjectList(msg.getQueriesList(),
    proto.pb.GetSlpTrustedValidationRequest.Query.toObject, includeInstance),
    includeGraphsearchCount: jspb.Message.getFieldWithDefault(msg, 2, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpTrustedValidationRequest}
 */
proto.pb.GetSlpTrustedValidationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpTrustedValidationRequest;
  return proto.pb.GetSlpTrustedValidationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpTrustedValidationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpTrustedValidationRequest}
 */
proto.pb.GetSlpTrustedValidationRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.GetSlpTrustedValidationRequest.Query;
      reader.readMessage(value,proto.pb.GetSlpTrustedValidationRequest.Query.deserializeBinaryFromReader);
      msg.addQueries(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeGraphsearchCount(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTrustedValidationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpTrustedValidationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpTrustedValidationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTrustedValidationRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQueriesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.pb.GetSlpTrustedValidationRequest.Query.serializeBinaryToWriter
    );
  }
  f = message.getIncludeGraphsearchCount();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpTrustedValidationRequest.Query = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetSlpTrustedValidationRequest.Query.repeatedFields_, null);
};
goog.inherits(proto.pb.GetSlpTrustedValidationRequest.Query, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpTrustedValidationRequest.Query.displayName = 'proto.pb.GetSlpTrustedValidationRequest.Query';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetSlpTrustedValidationRequest.Query.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpTrustedValidationRequest.Query.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpTrustedValidationRequest.Query} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTrustedValidationRequest.Query.toObject = function(includeInstance, msg) {
  var f, obj = {
    prevOutHash: msg.getPrevOutHash_asB64(),
    prevOutVout: jspb.Message.getFieldWithDefault(msg, 2, 0),
    graphsearchValidHashesList: msg.getGraphsearchValidHashesList_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpTrustedValidationRequest.Query}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpTrustedValidationRequest.Query;
  return proto.pb.GetSlpTrustedValidationRequest.Query.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpTrustedValidationRequest.Query} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpTrustedValidationRequest.Query}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPrevOutHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setPrevOutVout(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addGraphsearchValidHashes(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpTrustedValidationRequest.Query.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpTrustedValidationRequest.Query} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTrustedValidationRequest.Query.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPrevOutHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getPrevOutVout();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getGraphsearchValidHashesList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      3,
      f
    );
  }
};


/**
 * optional bytes prev_out_hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.getPrevOutHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes prev_out_hash = 1;
 * This is a type-conversion wrapper around `getPrevOutHash()`
 * @return {string}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.getPrevOutHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPrevOutHash()));
};


/**
 * optional bytes prev_out_hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPrevOutHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.getPrevOutHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPrevOutHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.setPrevOutHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional uint32 prev_out_vout = 2;
 * @return {number}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.getPrevOutVout = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.setPrevOutVout = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * repeated bytes graphsearch_valid_hashes = 3;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.getGraphsearchValidHashesList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * repeated bytes graphsearch_valid_hashes = 3;
 * This is a type-conversion wrapper around `getGraphsearchValidHashesList()`
 * @return {!Array<string>}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.getGraphsearchValidHashesList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getGraphsearchValidHashesList()));
};


/**
 * repeated bytes graphsearch_valid_hashes = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getGraphsearchValidHashesList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.getGraphsearchValidHashesList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getGraphsearchValidHashesList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.setGraphsearchValidHashesList = function(value) {
  jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.GetSlpTrustedValidationRequest.Query.prototype.addGraphsearchValidHashes = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


proto.pb.GetSlpTrustedValidationRequest.Query.prototype.clearGraphsearchValidHashesList = function() {
  this.setGraphsearchValidHashesList([]);
};


/**
 * repeated Query queries = 1;
 * @return {!Array<!proto.pb.GetSlpTrustedValidationRequest.Query>}
 */
proto.pb.GetSlpTrustedValidationRequest.prototype.getQueriesList = function() {
  return /** @type{!Array<!proto.pb.GetSlpTrustedValidationRequest.Query>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.GetSlpTrustedValidationRequest.Query, 1));
};


/** @param {!Array<!proto.pb.GetSlpTrustedValidationRequest.Query>} value */
proto.pb.GetSlpTrustedValidationRequest.prototype.setQueriesList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.pb.GetSlpTrustedValidationRequest.Query=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.GetSlpTrustedValidationRequest.Query}
 */
proto.pb.GetSlpTrustedValidationRequest.prototype.addQueries = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.pb.GetSlpTrustedValidationRequest.Query, opt_index);
};


proto.pb.GetSlpTrustedValidationRequest.prototype.clearQueriesList = function() {
  this.setQueriesList([]);
};


/**
 * optional bool include_graphsearch_count = 2;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetSlpTrustedValidationRequest.prototype.getIncludeGraphsearchCount = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 2, false));
};


/** @param {boolean} value */
proto.pb.GetSlpTrustedValidationRequest.prototype.setIncludeGraphsearchCount = function(value) {
  jspb.Message.setProto3BooleanField(this, 2, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpTrustedValidationResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetSlpTrustedValidationResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetSlpTrustedValidationResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpTrustedValidationResponse.displayName = 'proto.pb.GetSlpTrustedValidationResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetSlpTrustedValidationResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpTrustedValidationResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpTrustedValidationResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpTrustedValidationResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTrustedValidationResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    proto.pb.GetSlpTrustedValidationResponse.ValidityResult.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpTrustedValidationResponse}
 */
proto.pb.GetSlpTrustedValidationResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpTrustedValidationResponse;
  return proto.pb.GetSlpTrustedValidationResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpTrustedValidationResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpTrustedValidationResponse}
 */
proto.pb.GetSlpTrustedValidationResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.GetSlpTrustedValidationResponse.ValidityResult;
      reader.readMessage(value,proto.pb.GetSlpTrustedValidationResponse.ValidityResult.deserializeBinaryFromReader);
      msg.addResults(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTrustedValidationResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpTrustedValidationResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpTrustedValidationResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTrustedValidationResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.pb.GetSlpTrustedValidationResponse.ValidityResult.serializeBinaryToWriter
    );
  }
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.GetSlpTrustedValidationResponse.ValidityResult.oneofGroups_);
};
goog.inherits(proto.pb.GetSlpTrustedValidationResponse.ValidityResult, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpTrustedValidationResponse.ValidityResult.displayName = 'proto.pb.GetSlpTrustedValidationResponse.ValidityResult';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.oneofGroups_ = [[6,7]];

/**
 * @enum {number}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.ValidityResultTypeCase = {
  VALIDITY_RESULT_TYPE_NOT_SET: 0,
  V1_TOKEN_AMOUNT: 6,
  V1_MINT_BATON: 7
};

/**
 * @return {proto.pb.GetSlpTrustedValidationResponse.ValidityResult.ValidityResultTypeCase}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getValidityResultTypeCase = function() {
  return /** @type {proto.pb.GetSlpTrustedValidationResponse.ValidityResult.ValidityResultTypeCase} */(jspb.Message.computeOneofCase(this, proto.pb.GetSlpTrustedValidationResponse.ValidityResult.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpTrustedValidationResponse.ValidityResult.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpTrustedValidationResponse.ValidityResult} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.toObject = function(includeInstance, msg) {
  var f, obj = {
    prevOutHash: msg.getPrevOutHash_asB64(),
    prevOutVout: jspb.Message.getFieldWithDefault(msg, 2, 0),
    tokenId: msg.getTokenId_asB64(),
    slpAction: jspb.Message.getFieldWithDefault(msg, 4, 0),
    tokenType: jspb.Message.getFieldWithDefault(msg, 5, 0),
    v1TokenAmount: jspb.Message.getFieldWithDefault(msg, 6, "0"),
    v1MintBaton: jspb.Message.getFieldWithDefault(msg, 7, false),
    slpTxnOpreturn: msg.getSlpTxnOpreturn_asB64(),
    graphsearchTxnCount: jspb.Message.getFieldWithDefault(msg, 9, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpTrustedValidationResponse.ValidityResult}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpTrustedValidationResponse.ValidityResult;
  return proto.pb.GetSlpTrustedValidationResponse.ValidityResult.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpTrustedValidationResponse.ValidityResult} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpTrustedValidationResponse.ValidityResult}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPrevOutHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setPrevOutVout(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenId(value);
      break;
    case 4:
      var value = /** @type {!proto.pb.SlpAction} */ (reader.readEnum());
      msg.setSlpAction(value);
      break;
    case 5:
      var value = /** @type {!proto.pb.SlpTokenType} */ (reader.readEnum());
      msg.setTokenType(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readUint64String());
      msg.setV1TokenAmount(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setV1MintBaton(value);
      break;
    case 8:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSlpTxnOpreturn(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setGraphsearchTxnCount(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpTrustedValidationResponse.ValidityResult.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpTrustedValidationResponse.ValidityResult} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPrevOutHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getPrevOutVout();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getTokenId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getSlpAction();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getTokenType();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
  f = /** @type {string} */ (jspb.Message.getField(message, 6));
  if (f != null) {
    writer.writeUint64String(
      6,
      f
    );
  }
  f = /** @type {boolean} */ (jspb.Message.getField(message, 7));
  if (f != null) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getSlpTxnOpreturn_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      8,
      f
    );
  }
  f = message.getGraphsearchTxnCount();
  if (f !== 0) {
    writer.writeUint32(
      9,
      f
    );
  }
};


/**
 * optional bytes prev_out_hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getPrevOutHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes prev_out_hash = 1;
 * This is a type-conversion wrapper around `getPrevOutHash()`
 * @return {string}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getPrevOutHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPrevOutHash()));
};


/**
 * optional bytes prev_out_hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPrevOutHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getPrevOutHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPrevOutHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setPrevOutHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional uint32 prev_out_vout = 2;
 * @return {number}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getPrevOutVout = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setPrevOutVout = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional bytes token_id = 3;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getTokenId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes token_id = 3;
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {string}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getTokenId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenId()));
};


/**
 * optional bytes token_id = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getTokenId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setTokenId = function(value) {
  jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional SlpAction slp_action = 4;
 * @return {!proto.pb.SlpAction}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getSlpAction = function() {
  return /** @type {!proto.pb.SlpAction} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/** @param {!proto.pb.SlpAction} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setSlpAction = function(value) {
  jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional SlpTokenType token_type = 5;
 * @return {!proto.pb.SlpTokenType}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getTokenType = function() {
  return /** @type {!proto.pb.SlpTokenType} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {!proto.pb.SlpTokenType} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setTokenType = function(value) {
  jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional uint64 v1_token_amount = 6;
 * @return {string}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getV1TokenAmount = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, "0"));
};


/** @param {string} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setV1TokenAmount = function(value) {
  jspb.Message.setOneofField(this, 6, proto.pb.GetSlpTrustedValidationResponse.ValidityResult.oneofGroups_[0], value);
};


proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.clearV1TokenAmount = function() {
  jspb.Message.setOneofField(this, 6, proto.pb.GetSlpTrustedValidationResponse.ValidityResult.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.hasV1TokenAmount = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional bool v1_mint_baton = 7;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getV1MintBaton = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 7, false));
};


/** @param {boolean} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setV1MintBaton = function(value) {
  jspb.Message.setOneofField(this, 7, proto.pb.GetSlpTrustedValidationResponse.ValidityResult.oneofGroups_[0], value);
};


proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.clearV1MintBaton = function() {
  jspb.Message.setOneofField(this, 7, proto.pb.GetSlpTrustedValidationResponse.ValidityResult.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.hasV1MintBaton = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * optional bytes slp_txn_opreturn = 8;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getSlpTxnOpreturn = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * optional bytes slp_txn_opreturn = 8;
 * This is a type-conversion wrapper around `getSlpTxnOpreturn()`
 * @return {string}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getSlpTxnOpreturn_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSlpTxnOpreturn()));
};


/**
 * optional bytes slp_txn_opreturn = 8;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSlpTxnOpreturn()`
 * @return {!Uint8Array}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getSlpTxnOpreturn_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSlpTxnOpreturn()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setSlpTxnOpreturn = function(value) {
  jspb.Message.setProto3BytesField(this, 8, value);
};


/**
 * optional uint32 graphsearch_txn_count = 9;
 * @return {number}
 */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.getGraphsearchTxnCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/** @param {number} value */
proto.pb.GetSlpTrustedValidationResponse.ValidityResult.prototype.setGraphsearchTxnCount = function(value) {
  jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * repeated ValidityResult results = 1;
 * @return {!Array<!proto.pb.GetSlpTrustedValidationResponse.ValidityResult>}
 */
proto.pb.GetSlpTrustedValidationResponse.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.pb.GetSlpTrustedValidationResponse.ValidityResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.GetSlpTrustedValidationResponse.ValidityResult, 1));
};


/** @param {!Array<!proto.pb.GetSlpTrustedValidationResponse.ValidityResult>} value */
proto.pb.GetSlpTrustedValidationResponse.prototype.setResultsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.pb.GetSlpTrustedValidationResponse.ValidityResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.GetSlpTrustedValidationResponse.ValidityResult}
 */
proto.pb.GetSlpTrustedValidationResponse.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.pb.GetSlpTrustedValidationResponse.ValidityResult, opt_index);
};


proto.pb.GetSlpTrustedValidationResponse.prototype.clearResultsList = function() {
  this.setResultsList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpGraphSearchRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetSlpGraphSearchRequest.repeatedFields_, null);
};
goog.inherits(proto.pb.GetSlpGraphSearchRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpGraphSearchRequest.displayName = 'proto.pb.GetSlpGraphSearchRequest';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetSlpGraphSearchRequest.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpGraphSearchRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpGraphSearchRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpGraphSearchRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpGraphSearchRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    validHashesList: msg.getValidHashesList_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpGraphSearchRequest}
 */
proto.pb.GetSlpGraphSearchRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpGraphSearchRequest;
  return proto.pb.GetSlpGraphSearchRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpGraphSearchRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpGraphSearchRequest}
 */
proto.pb.GetSlpGraphSearchRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addValidHashes(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpGraphSearchRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpGraphSearchRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpGraphSearchRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpGraphSearchRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getValidHashesList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      2,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.GetSlpGraphSearchRequest.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.GetSlpGraphSearchRequest.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.GetSlpGraphSearchRequest.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.GetSlpGraphSearchRequest.prototype.setHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * repeated bytes valid_hashes = 2;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.GetSlpGraphSearchRequest.prototype.getValidHashesList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * repeated bytes valid_hashes = 2;
 * This is a type-conversion wrapper around `getValidHashesList()`
 * @return {!Array<string>}
 */
proto.pb.GetSlpGraphSearchRequest.prototype.getValidHashesList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getValidHashesList()));
};


/**
 * repeated bytes valid_hashes = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getValidHashesList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.GetSlpGraphSearchRequest.prototype.getValidHashesList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getValidHashesList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.GetSlpGraphSearchRequest.prototype.setValidHashesList = function(value) {
  jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.GetSlpGraphSearchRequest.prototype.addValidHashes = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


proto.pb.GetSlpGraphSearchRequest.prototype.clearValidHashesList = function() {
  this.setValidHashesList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.GetSlpGraphSearchResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.GetSlpGraphSearchResponse.repeatedFields_, null);
};
goog.inherits(proto.pb.GetSlpGraphSearchResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.GetSlpGraphSearchResponse.displayName = 'proto.pb.GetSlpGraphSearchResponse';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.GetSlpGraphSearchResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.GetSlpGraphSearchResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.GetSlpGraphSearchResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.GetSlpGraphSearchResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpGraphSearchResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    txdataList: msg.getTxdataList_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.GetSlpGraphSearchResponse}
 */
proto.pb.GetSlpGraphSearchResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.GetSlpGraphSearchResponse;
  return proto.pb.GetSlpGraphSearchResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.GetSlpGraphSearchResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.GetSlpGraphSearchResponse}
 */
proto.pb.GetSlpGraphSearchResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addTxdata(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.GetSlpGraphSearchResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.GetSlpGraphSearchResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.GetSlpGraphSearchResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.GetSlpGraphSearchResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTxdataList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      1,
      f
    );
  }
};


/**
 * repeated bytes txdata = 1;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.GetSlpGraphSearchResponse.prototype.getTxdataList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * repeated bytes txdata = 1;
 * This is a type-conversion wrapper around `getTxdataList()`
 * @return {!Array<string>}
 */
proto.pb.GetSlpGraphSearchResponse.prototype.getTxdataList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getTxdataList()));
};


/**
 * repeated bytes txdata = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTxdataList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.GetSlpGraphSearchResponse.prototype.getTxdataList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getTxdataList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.GetSlpGraphSearchResponse.prototype.setTxdataList = function(value) {
  jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.GetSlpGraphSearchResponse.prototype.addTxdata = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


proto.pb.GetSlpGraphSearchResponse.prototype.clearTxdataList = function() {
  this.setTxdataList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.BlockNotification = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.BlockNotification.oneofGroups_);
};
goog.inherits(proto.pb.BlockNotification, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.BlockNotification.displayName = 'proto.pb.BlockNotification';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.BlockNotification.oneofGroups_ = [[2,3,4]];

/**
 * @enum {number}
 */
proto.pb.BlockNotification.BlockCase = {
  BLOCK_NOT_SET: 0,
  BLOCK_INFO: 2,
  MARSHALED_BLOCK: 3,
  SERIALIZED_BLOCK: 4
};

/**
 * @return {proto.pb.BlockNotification.BlockCase}
 */
proto.pb.BlockNotification.prototype.getBlockCase = function() {
  return /** @type {proto.pb.BlockNotification.BlockCase} */(jspb.Message.computeOneofCase(this, proto.pb.BlockNotification.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.BlockNotification.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.BlockNotification.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.BlockNotification} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.BlockNotification.toObject = function(includeInstance, msg) {
  var f, obj = {
    type: jspb.Message.getFieldWithDefault(msg, 1, 0),
    blockInfo: (f = msg.getBlockInfo()) && proto.pb.BlockInfo.toObject(includeInstance, f),
    marshaledBlock: (f = msg.getMarshaledBlock()) && proto.pb.Block.toObject(includeInstance, f),
    serializedBlock: msg.getSerializedBlock_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.BlockNotification}
 */
proto.pb.BlockNotification.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.BlockNotification;
  return proto.pb.BlockNotification.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.BlockNotification} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.BlockNotification}
 */
proto.pb.BlockNotification.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.pb.BlockNotification.Type} */ (reader.readEnum());
      msg.setType(value);
      break;
    case 2:
      var value = new proto.pb.BlockInfo;
      reader.readMessage(value,proto.pb.BlockInfo.deserializeBinaryFromReader);
      msg.setBlockInfo(value);
      break;
    case 3:
      var value = new proto.pb.Block;
      reader.readMessage(value,proto.pb.Block.deserializeBinaryFromReader);
      msg.setMarshaledBlock(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSerializedBlock(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.BlockNotification.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.BlockNotification.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.BlockNotification} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.BlockNotification.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getType();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getBlockInfo();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.pb.BlockInfo.serializeBinaryToWriter
    );
  }
  f = message.getMarshaledBlock();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.pb.Block.serializeBinaryToWriter
    );
  }
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 4));
  if (f != null) {
    writer.writeBytes(
      4,
      f
    );
  }
};


/**
 * @enum {number}
 */
proto.pb.BlockNotification.Type = {
  CONNECTED: 0,
  DISCONNECTED: 1
};

/**
 * optional Type type = 1;
 * @return {!proto.pb.BlockNotification.Type}
 */
proto.pb.BlockNotification.prototype.getType = function() {
  return /** @type {!proto.pb.BlockNotification.Type} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {!proto.pb.BlockNotification.Type} value */
proto.pb.BlockNotification.prototype.setType = function(value) {
  jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional BlockInfo block_info = 2;
 * @return {?proto.pb.BlockInfo}
 */
proto.pb.BlockNotification.prototype.getBlockInfo = function() {
  return /** @type{?proto.pb.BlockInfo} */ (
    jspb.Message.getWrapperField(this, proto.pb.BlockInfo, 2));
};


/** @param {?proto.pb.BlockInfo|undefined} value */
proto.pb.BlockNotification.prototype.setBlockInfo = function(value) {
  jspb.Message.setOneofWrapperField(this, 2, proto.pb.BlockNotification.oneofGroups_[0], value);
};


proto.pb.BlockNotification.prototype.clearBlockInfo = function() {
  this.setBlockInfo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.BlockNotification.prototype.hasBlockInfo = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional Block marshaled_block = 3;
 * @return {?proto.pb.Block}
 */
proto.pb.BlockNotification.prototype.getMarshaledBlock = function() {
  return /** @type{?proto.pb.Block} */ (
    jspb.Message.getWrapperField(this, proto.pb.Block, 3));
};


/** @param {?proto.pb.Block|undefined} value */
proto.pb.BlockNotification.prototype.setMarshaledBlock = function(value) {
  jspb.Message.setOneofWrapperField(this, 3, proto.pb.BlockNotification.oneofGroups_[0], value);
};


proto.pb.BlockNotification.prototype.clearMarshaledBlock = function() {
  this.setMarshaledBlock(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.BlockNotification.prototype.hasMarshaledBlock = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional bytes serialized_block = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.BlockNotification.prototype.getSerializedBlock = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes serialized_block = 4;
 * This is a type-conversion wrapper around `getSerializedBlock()`
 * @return {string}
 */
proto.pb.BlockNotification.prototype.getSerializedBlock_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSerializedBlock()));
};


/**
 * optional bytes serialized_block = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSerializedBlock()`
 * @return {!Uint8Array}
 */
proto.pb.BlockNotification.prototype.getSerializedBlock_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSerializedBlock()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.BlockNotification.prototype.setSerializedBlock = function(value) {
  jspb.Message.setOneofField(this, 4, proto.pb.BlockNotification.oneofGroups_[0], value);
};


proto.pb.BlockNotification.prototype.clearSerializedBlock = function() {
  jspb.Message.setOneofField(this, 4, proto.pb.BlockNotification.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.BlockNotification.prototype.hasSerializedBlock = function() {
  return jspb.Message.getField(this, 4) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.TransactionNotification = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.TransactionNotification.oneofGroups_);
};
goog.inherits(proto.pb.TransactionNotification, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.TransactionNotification.displayName = 'proto.pb.TransactionNotification';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.TransactionNotification.oneofGroups_ = [[2,3,4]];

/**
 * @enum {number}
 */
proto.pb.TransactionNotification.TransactionCase = {
  TRANSACTION_NOT_SET: 0,
  CONFIRMED_TRANSACTION: 2,
  UNCONFIRMED_TRANSACTION: 3,
  SERIALIZED_TRANSACTION: 4
};

/**
 * @return {proto.pb.TransactionNotification.TransactionCase}
 */
proto.pb.TransactionNotification.prototype.getTransactionCase = function() {
  return /** @type {proto.pb.TransactionNotification.TransactionCase} */(jspb.Message.computeOneofCase(this, proto.pb.TransactionNotification.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.TransactionNotification.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.TransactionNotification.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.TransactionNotification} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.TransactionNotification.toObject = function(includeInstance, msg) {
  var f, obj = {
    type: jspb.Message.getFieldWithDefault(msg, 1, 0),
    confirmedTransaction: (f = msg.getConfirmedTransaction()) && proto.pb.Transaction.toObject(includeInstance, f),
    unconfirmedTransaction: (f = msg.getUnconfirmedTransaction()) && proto.pb.MempoolTransaction.toObject(includeInstance, f),
    serializedTransaction: msg.getSerializedTransaction_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.TransactionNotification}
 */
proto.pb.TransactionNotification.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.TransactionNotification;
  return proto.pb.TransactionNotification.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.TransactionNotification} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.TransactionNotification}
 */
proto.pb.TransactionNotification.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.pb.TransactionNotification.Type} */ (reader.readEnum());
      msg.setType(value);
      break;
    case 2:
      var value = new proto.pb.Transaction;
      reader.readMessage(value,proto.pb.Transaction.deserializeBinaryFromReader);
      msg.setConfirmedTransaction(value);
      break;
    case 3:
      var value = new proto.pb.MempoolTransaction;
      reader.readMessage(value,proto.pb.MempoolTransaction.deserializeBinaryFromReader);
      msg.setUnconfirmedTransaction(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSerializedTransaction(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.TransactionNotification.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.TransactionNotification.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.TransactionNotification} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.TransactionNotification.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getType();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getConfirmedTransaction();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.pb.Transaction.serializeBinaryToWriter
    );
  }
  f = message.getUnconfirmedTransaction();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.pb.MempoolTransaction.serializeBinaryToWriter
    );
  }
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 4));
  if (f != null) {
    writer.writeBytes(
      4,
      f
    );
  }
};


/**
 * @enum {number}
 */
proto.pb.TransactionNotification.Type = {
  UNCONFIRMED: 0,
  CONFIRMED: 1
};

/**
 * optional Type type = 1;
 * @return {!proto.pb.TransactionNotification.Type}
 */
proto.pb.TransactionNotification.prototype.getType = function() {
  return /** @type {!proto.pb.TransactionNotification.Type} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {!proto.pb.TransactionNotification.Type} value */
proto.pb.TransactionNotification.prototype.setType = function(value) {
  jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional Transaction confirmed_transaction = 2;
 * @return {?proto.pb.Transaction}
 */
proto.pb.TransactionNotification.prototype.getConfirmedTransaction = function() {
  return /** @type{?proto.pb.Transaction} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction, 2));
};


/** @param {?proto.pb.Transaction|undefined} value */
proto.pb.TransactionNotification.prototype.setConfirmedTransaction = function(value) {
  jspb.Message.setOneofWrapperField(this, 2, proto.pb.TransactionNotification.oneofGroups_[0], value);
};


proto.pb.TransactionNotification.prototype.clearConfirmedTransaction = function() {
  this.setConfirmedTransaction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.TransactionNotification.prototype.hasConfirmedTransaction = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional MempoolTransaction unconfirmed_transaction = 3;
 * @return {?proto.pb.MempoolTransaction}
 */
proto.pb.TransactionNotification.prototype.getUnconfirmedTransaction = function() {
  return /** @type{?proto.pb.MempoolTransaction} */ (
    jspb.Message.getWrapperField(this, proto.pb.MempoolTransaction, 3));
};


/** @param {?proto.pb.MempoolTransaction|undefined} value */
proto.pb.TransactionNotification.prototype.setUnconfirmedTransaction = function(value) {
  jspb.Message.setOneofWrapperField(this, 3, proto.pb.TransactionNotification.oneofGroups_[0], value);
};


proto.pb.TransactionNotification.prototype.clearUnconfirmedTransaction = function() {
  this.setUnconfirmedTransaction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.TransactionNotification.prototype.hasUnconfirmedTransaction = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional bytes serialized_transaction = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.TransactionNotification.prototype.getSerializedTransaction = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes serialized_transaction = 4;
 * This is a type-conversion wrapper around `getSerializedTransaction()`
 * @return {string}
 */
proto.pb.TransactionNotification.prototype.getSerializedTransaction_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSerializedTransaction()));
};


/**
 * optional bytes serialized_transaction = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSerializedTransaction()`
 * @return {!Uint8Array}
 */
proto.pb.TransactionNotification.prototype.getSerializedTransaction_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSerializedTransaction()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.TransactionNotification.prototype.setSerializedTransaction = function(value) {
  jspb.Message.setOneofField(this, 4, proto.pb.TransactionNotification.oneofGroups_[0], value);
};


proto.pb.TransactionNotification.prototype.clearSerializedTransaction = function() {
  jspb.Message.setOneofField(this, 4, proto.pb.TransactionNotification.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.TransactionNotification.prototype.hasSerializedTransaction = function() {
  return jspb.Message.getField(this, 4) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.BlockInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.BlockInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.BlockInfo.displayName = 'proto.pb.BlockInfo';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.BlockInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.BlockInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.BlockInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.BlockInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    height: jspb.Message.getFieldWithDefault(msg, 2, 0),
    version: jspb.Message.getFieldWithDefault(msg, 3, 0),
    previousBlock: msg.getPreviousBlock_asB64(),
    merkleRoot: msg.getMerkleRoot_asB64(),
    timestamp: jspb.Message.getFieldWithDefault(msg, 6, 0),
    bits: jspb.Message.getFieldWithDefault(msg, 7, 0),
    nonce: jspb.Message.getFieldWithDefault(msg, 8, 0),
    confirmations: jspb.Message.getFieldWithDefault(msg, 9, 0),
    difficulty: +jspb.Message.getFieldWithDefault(msg, 10, 0.0),
    nextBlockHash: msg.getNextBlockHash_asB64(),
    size: jspb.Message.getFieldWithDefault(msg, 12, 0),
    medianTime: jspb.Message.getFieldWithDefault(msg, 13, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.BlockInfo}
 */
proto.pb.BlockInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.BlockInfo;
  return proto.pb.BlockInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.BlockInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.BlockInfo}
 */
proto.pb.BlockInfo.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHeight(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setVersion(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPreviousBlock(value);
      break;
    case 5:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setMerkleRoot(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTimestamp(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setBits(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setNonce(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setConfirmations(value);
      break;
    case 10:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setDifficulty(value);
      break;
    case 11:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setNextBlockHash(value);
      break;
    case 12:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSize(value);
      break;
    case 13:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setMedianTime(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.BlockInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.BlockInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.BlockInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.BlockInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getHeight();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getVersion();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getPreviousBlock_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getMerkleRoot_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      5,
      f
    );
  }
  f = message.getTimestamp();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
  f = message.getBits();
  if (f !== 0) {
    writer.writeUint32(
      7,
      f
    );
  }
  f = message.getNonce();
  if (f !== 0) {
    writer.writeUint32(
      8,
      f
    );
  }
  f = message.getConfirmations();
  if (f !== 0) {
    writer.writeInt32(
      9,
      f
    );
  }
  f = message.getDifficulty();
  if (f !== 0.0) {
    writer.writeDouble(
      10,
      f
    );
  }
  f = message.getNextBlockHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      11,
      f
    );
  }
  f = message.getSize();
  if (f !== 0) {
    writer.writeInt32(
      12,
      f
    );
  }
  f = message.getMedianTime();
  if (f !== 0) {
    writer.writeInt64(
      13,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.BlockInfo.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.BlockInfo.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.BlockInfo.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.BlockInfo.prototype.setHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional int32 height = 2;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setHeight = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int32 version = 3;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getVersion = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setVersion = function(value) {
  jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional bytes previous_block = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.BlockInfo.prototype.getPreviousBlock = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes previous_block = 4;
 * This is a type-conversion wrapper around `getPreviousBlock()`
 * @return {string}
 */
proto.pb.BlockInfo.prototype.getPreviousBlock_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPreviousBlock()));
};


/**
 * optional bytes previous_block = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPreviousBlock()`
 * @return {!Uint8Array}
 */
proto.pb.BlockInfo.prototype.getPreviousBlock_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPreviousBlock()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.BlockInfo.prototype.setPreviousBlock = function(value) {
  jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * optional bytes merkle_root = 5;
 * @return {!(string|Uint8Array)}
 */
proto.pb.BlockInfo.prototype.getMerkleRoot = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * optional bytes merkle_root = 5;
 * This is a type-conversion wrapper around `getMerkleRoot()`
 * @return {string}
 */
proto.pb.BlockInfo.prototype.getMerkleRoot_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getMerkleRoot()));
};


/**
 * optional bytes merkle_root = 5;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getMerkleRoot()`
 * @return {!Uint8Array}
 */
proto.pb.BlockInfo.prototype.getMerkleRoot_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getMerkleRoot()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.BlockInfo.prototype.setMerkleRoot = function(value) {
  jspb.Message.setProto3BytesField(this, 5, value);
};


/**
 * optional int64 timestamp = 6;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getTimestamp = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setTimestamp = function(value) {
  jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional uint32 bits = 7;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getBits = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setBits = function(value) {
  jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional uint32 nonce = 8;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getNonce = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setNonce = function(value) {
  jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional int32 confirmations = 9;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getConfirmations = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setConfirmations = function(value) {
  jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * optional double difficulty = 10;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getDifficulty = function() {
  return /** @type {number} */ (+jspb.Message.getFieldWithDefault(this, 10, 0.0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setDifficulty = function(value) {
  jspb.Message.setProto3FloatField(this, 10, value);
};


/**
 * optional bytes next_block_hash = 11;
 * @return {!(string|Uint8Array)}
 */
proto.pb.BlockInfo.prototype.getNextBlockHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * optional bytes next_block_hash = 11;
 * This is a type-conversion wrapper around `getNextBlockHash()`
 * @return {string}
 */
proto.pb.BlockInfo.prototype.getNextBlockHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getNextBlockHash()));
};


/**
 * optional bytes next_block_hash = 11;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getNextBlockHash()`
 * @return {!Uint8Array}
 */
proto.pb.BlockInfo.prototype.getNextBlockHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getNextBlockHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.BlockInfo.prototype.setNextBlockHash = function(value) {
  jspb.Message.setProto3BytesField(this, 11, value);
};


/**
 * optional int32 size = 12;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 12, 0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setSize = function(value) {
  jspb.Message.setProto3IntField(this, 12, value);
};


/**
 * optional int64 median_time = 13;
 * @return {number}
 */
proto.pb.BlockInfo.prototype.getMedianTime = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 13, 0));
};


/** @param {number} value */
proto.pb.BlockInfo.prototype.setMedianTime = function(value) {
  jspb.Message.setProto3IntField(this, 13, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.Block = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.Block.repeatedFields_, null);
};
goog.inherits(proto.pb.Block, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.Block.displayName = 'proto.pb.Block';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.Block.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.Block.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.Block.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.Block} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Block.toObject = function(includeInstance, msg) {
  var f, obj = {
    info: (f = msg.getInfo()) && proto.pb.BlockInfo.toObject(includeInstance, f),
    transactionDataList: jspb.Message.toObjectList(msg.getTransactionDataList(),
    proto.pb.Block.TransactionData.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.Block}
 */
proto.pb.Block.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.Block;
  return proto.pb.Block.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.Block} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.Block}
 */
proto.pb.Block.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.BlockInfo;
      reader.readMessage(value,proto.pb.BlockInfo.deserializeBinaryFromReader);
      msg.setInfo(value);
      break;
    case 2:
      var value = new proto.pb.Block.TransactionData;
      reader.readMessage(value,proto.pb.Block.TransactionData.deserializeBinaryFromReader);
      msg.addTransactionData(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.Block.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.Block.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.Block} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Block.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getInfo();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.BlockInfo.serializeBinaryToWriter
    );
  }
  f = message.getTransactionDataList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.pb.Block.TransactionData.serializeBinaryToWriter
    );
  }
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.Block.TransactionData = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.Block.TransactionData.oneofGroups_);
};
goog.inherits(proto.pb.Block.TransactionData, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.Block.TransactionData.displayName = 'proto.pb.Block.TransactionData';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.Block.TransactionData.oneofGroups_ = [[1,2]];

/**
 * @enum {number}
 */
proto.pb.Block.TransactionData.TxidsOrTxsCase = {
  TXIDS_OR_TXS_NOT_SET: 0,
  TRANSACTION_HASH: 1,
  TRANSACTION: 2
};

/**
 * @return {proto.pb.Block.TransactionData.TxidsOrTxsCase}
 */
proto.pb.Block.TransactionData.prototype.getTxidsOrTxsCase = function() {
  return /** @type {proto.pb.Block.TransactionData.TxidsOrTxsCase} */(jspb.Message.computeOneofCase(this, proto.pb.Block.TransactionData.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.Block.TransactionData.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.Block.TransactionData.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.Block.TransactionData} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Block.TransactionData.toObject = function(includeInstance, msg) {
  var f, obj = {
    transactionHash: msg.getTransactionHash_asB64(),
    transaction: (f = msg.getTransaction()) && proto.pb.Transaction.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.Block.TransactionData}
 */
proto.pb.Block.TransactionData.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.Block.TransactionData;
  return proto.pb.Block.TransactionData.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.Block.TransactionData} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.Block.TransactionData}
 */
proto.pb.Block.TransactionData.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTransactionHash(value);
      break;
    case 2:
      var value = new proto.pb.Transaction;
      reader.readMessage(value,proto.pb.Transaction.deserializeBinaryFromReader);
      msg.setTransaction(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.Block.TransactionData.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.Block.TransactionData.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.Block.TransactionData} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Block.TransactionData.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 1));
  if (f != null) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getTransaction();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.pb.Transaction.serializeBinaryToWriter
    );
  }
};


/**
 * optional bytes transaction_hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.Block.TransactionData.prototype.getTransactionHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes transaction_hash = 1;
 * This is a type-conversion wrapper around `getTransactionHash()`
 * @return {string}
 */
proto.pb.Block.TransactionData.prototype.getTransactionHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTransactionHash()));
};


/**
 * optional bytes transaction_hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTransactionHash()`
 * @return {!Uint8Array}
 */
proto.pb.Block.TransactionData.prototype.getTransactionHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTransactionHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.Block.TransactionData.prototype.setTransactionHash = function(value) {
  jspb.Message.setOneofField(this, 1, proto.pb.Block.TransactionData.oneofGroups_[0], value);
};


proto.pb.Block.TransactionData.prototype.clearTransactionHash = function() {
  jspb.Message.setOneofField(this, 1, proto.pb.Block.TransactionData.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Block.TransactionData.prototype.hasTransactionHash = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional Transaction transaction = 2;
 * @return {?proto.pb.Transaction}
 */
proto.pb.Block.TransactionData.prototype.getTransaction = function() {
  return /** @type{?proto.pb.Transaction} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction, 2));
};


/** @param {?proto.pb.Transaction|undefined} value */
proto.pb.Block.TransactionData.prototype.setTransaction = function(value) {
  jspb.Message.setOneofWrapperField(this, 2, proto.pb.Block.TransactionData.oneofGroups_[0], value);
};


proto.pb.Block.TransactionData.prototype.clearTransaction = function() {
  this.setTransaction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Block.TransactionData.prototype.hasTransaction = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional BlockInfo info = 1;
 * @return {?proto.pb.BlockInfo}
 */
proto.pb.Block.prototype.getInfo = function() {
  return /** @type{?proto.pb.BlockInfo} */ (
    jspb.Message.getWrapperField(this, proto.pb.BlockInfo, 1));
};


/** @param {?proto.pb.BlockInfo|undefined} value */
proto.pb.Block.prototype.setInfo = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.Block.prototype.clearInfo = function() {
  this.setInfo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Block.prototype.hasInfo = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * repeated TransactionData transaction_data = 2;
 * @return {!Array<!proto.pb.Block.TransactionData>}
 */
proto.pb.Block.prototype.getTransactionDataList = function() {
  return /** @type{!Array<!proto.pb.Block.TransactionData>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.Block.TransactionData, 2));
};


/** @param {!Array<!proto.pb.Block.TransactionData>} value */
proto.pb.Block.prototype.setTransactionDataList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.pb.Block.TransactionData=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.Block.TransactionData}
 */
proto.pb.Block.prototype.addTransactionData = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.pb.Block.TransactionData, opt_index);
};


proto.pb.Block.prototype.clearTransactionDataList = function() {
  this.setTransactionDataList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.Transaction = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.Transaction.repeatedFields_, null);
};
goog.inherits(proto.pb.Transaction, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.Transaction.displayName = 'proto.pb.Transaction';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.Transaction.repeatedFields_ = [3,4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.Transaction.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.Transaction.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.Transaction} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Transaction.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    version: jspb.Message.getFieldWithDefault(msg, 2, 0),
    inputsList: jspb.Message.toObjectList(msg.getInputsList(),
    proto.pb.Transaction.Input.toObject, includeInstance),
    outputsList: jspb.Message.toObjectList(msg.getOutputsList(),
    proto.pb.Transaction.Output.toObject, includeInstance),
    lockTime: jspb.Message.getFieldWithDefault(msg, 5, 0),
    size: jspb.Message.getFieldWithDefault(msg, 8, 0),
    timestamp: jspb.Message.getFieldWithDefault(msg, 9, 0),
    confirmations: jspb.Message.getFieldWithDefault(msg, 10, 0),
    blockHeight: jspb.Message.getFieldWithDefault(msg, 11, 0),
    blockHash: msg.getBlockHash_asB64(),
    slpTransactionInfo: (f = msg.getSlpTransactionInfo()) && proto.pb.SlpTransactionInfo.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.Transaction}
 */
proto.pb.Transaction.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.Transaction;
  return proto.pb.Transaction.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.Transaction} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.Transaction}
 */
proto.pb.Transaction.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setVersion(value);
      break;
    case 3:
      var value = new proto.pb.Transaction.Input;
      reader.readMessage(value,proto.pb.Transaction.Input.deserializeBinaryFromReader);
      msg.addInputs(value);
      break;
    case 4:
      var value = new proto.pb.Transaction.Output;
      reader.readMessage(value,proto.pb.Transaction.Output.deserializeBinaryFromReader);
      msg.addOutputs(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setLockTime(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSize(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTimestamp(value);
      break;
    case 10:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setConfirmations(value);
      break;
    case 11:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setBlockHeight(value);
      break;
    case 12:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setBlockHash(value);
      break;
    case 13:
      var value = new proto.pb.SlpTransactionInfo;
      reader.readMessage(value,proto.pb.SlpTransactionInfo.deserializeBinaryFromReader);
      msg.setSlpTransactionInfo(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.Transaction.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.Transaction.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.Transaction} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Transaction.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getVersion();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getInputsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.pb.Transaction.Input.serializeBinaryToWriter
    );
  }
  f = message.getOutputsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.pb.Transaction.Output.serializeBinaryToWriter
    );
  }
  f = message.getLockTime();
  if (f !== 0) {
    writer.writeUint32(
      5,
      f
    );
  }
  f = message.getSize();
  if (f !== 0) {
    writer.writeInt32(
      8,
      f
    );
  }
  f = message.getTimestamp();
  if (f !== 0) {
    writer.writeInt64(
      9,
      f
    );
  }
  f = message.getConfirmations();
  if (f !== 0) {
    writer.writeInt32(
      10,
      f
    );
  }
  f = message.getBlockHeight();
  if (f !== 0) {
    writer.writeInt32(
      11,
      f
    );
  }
  f = message.getBlockHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      12,
      f
    );
  }
  f = message.getSlpTransactionInfo();
  if (f != null) {
    writer.writeMessage(
      13,
      f,
      proto.pb.SlpTransactionInfo.serializeBinaryToWriter
    );
  }
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.Transaction.Input = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.Transaction.Input, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.Transaction.Input.displayName = 'proto.pb.Transaction.Input';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.Transaction.Input.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.Transaction.Input.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.Transaction.Input} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Transaction.Input.toObject = function(includeInstance, msg) {
  var f, obj = {
    index: jspb.Message.getFieldWithDefault(msg, 1, 0),
    outpoint: (f = msg.getOutpoint()) && proto.pb.Transaction.Input.Outpoint.toObject(includeInstance, f),
    signatureScript: msg.getSignatureScript_asB64(),
    sequence: jspb.Message.getFieldWithDefault(msg, 4, 0),
    value: jspb.Message.getFieldWithDefault(msg, 5, 0),
    previousScript: msg.getPreviousScript_asB64(),
    address: jspb.Message.getFieldWithDefault(msg, 7, ""),
    slpToken: (f = msg.getSlpToken()) && proto.pb.SlpToken.toObject(includeInstance, f),
    cashToken: (f = msg.getCashToken()) && proto.pb.CashToken.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.Transaction.Input}
 */
proto.pb.Transaction.Input.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.Transaction.Input;
  return proto.pb.Transaction.Input.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.Transaction.Input} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.Transaction.Input}
 */
proto.pb.Transaction.Input.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setIndex(value);
      break;
    case 2:
      var value = new proto.pb.Transaction.Input.Outpoint;
      reader.readMessage(value,proto.pb.Transaction.Input.Outpoint.deserializeBinaryFromReader);
      msg.setOutpoint(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSignatureScript(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setSequence(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setValue(value);
      break;
    case 6:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPreviousScript(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setAddress(value);
      break;
    case 8:
      var value = new proto.pb.SlpToken;
      reader.readMessage(value,proto.pb.SlpToken.deserializeBinaryFromReader);
      msg.setSlpToken(value);
      break;
    case 9:
      var value = new proto.pb.CashToken;
      reader.readMessage(value,proto.pb.CashToken.deserializeBinaryFromReader);
      msg.setCashToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.Transaction.Input.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.Transaction.Input.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.Transaction.Input} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Transaction.Input.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIndex();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getOutpoint();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.pb.Transaction.Input.Outpoint.serializeBinaryToWriter
    );
  }
  f = message.getSignatureScript_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getSequence();
  if (f !== 0) {
    writer.writeUint32(
      4,
      f
    );
  }
  f = message.getValue();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getPreviousScript_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      6,
      f
    );
  }
  f = message.getAddress();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getSlpToken();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      proto.pb.SlpToken.serializeBinaryToWriter
    );
  }
  f = message.getCashToken();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      proto.pb.CashToken.serializeBinaryToWriter
    );
  }
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.Transaction.Input.Outpoint = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.Transaction.Input.Outpoint, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.Transaction.Input.Outpoint.displayName = 'proto.pb.Transaction.Input.Outpoint';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.Transaction.Input.Outpoint.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.Transaction.Input.Outpoint.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.Transaction.Input.Outpoint} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Transaction.Input.Outpoint.toObject = function(includeInstance, msg) {
  var f, obj = {
    hash: msg.getHash_asB64(),
    index: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.Transaction.Input.Outpoint}
 */
proto.pb.Transaction.Input.Outpoint.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.Transaction.Input.Outpoint;
  return proto.pb.Transaction.Input.Outpoint.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.Transaction.Input.Outpoint} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.Transaction.Input.Outpoint}
 */
proto.pb.Transaction.Input.Outpoint.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setHash(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setIndex(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.Transaction.Input.Outpoint.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.Transaction.Input.Outpoint.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.Transaction.Input.Outpoint} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Transaction.Input.Outpoint.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getIndex();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.Transaction.Input.Outpoint.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.Transaction.Input.Outpoint.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.Transaction.Input.Outpoint.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.Transaction.Input.Outpoint.prototype.setHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional uint32 index = 2;
 * @return {number}
 */
proto.pb.Transaction.Input.Outpoint.prototype.getIndex = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.Transaction.Input.Outpoint.prototype.setIndex = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional uint32 index = 1;
 * @return {number}
 */
proto.pb.Transaction.Input.prototype.getIndex = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {number} value */
proto.pb.Transaction.Input.prototype.setIndex = function(value) {
  jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional Outpoint outpoint = 2;
 * @return {?proto.pb.Transaction.Input.Outpoint}
 */
proto.pb.Transaction.Input.prototype.getOutpoint = function() {
  return /** @type{?proto.pb.Transaction.Input.Outpoint} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction.Input.Outpoint, 2));
};


/** @param {?proto.pb.Transaction.Input.Outpoint|undefined} value */
proto.pb.Transaction.Input.prototype.setOutpoint = function(value) {
  jspb.Message.setWrapperField(this, 2, value);
};


proto.pb.Transaction.Input.prototype.clearOutpoint = function() {
  this.setOutpoint(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Transaction.Input.prototype.hasOutpoint = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bytes signature_script = 3;
 * @return {!(string|Uint8Array)}
 */
proto.pb.Transaction.Input.prototype.getSignatureScript = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes signature_script = 3;
 * This is a type-conversion wrapper around `getSignatureScript()`
 * @return {string}
 */
proto.pb.Transaction.Input.prototype.getSignatureScript_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSignatureScript()));
};


/**
 * optional bytes signature_script = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSignatureScript()`
 * @return {!Uint8Array}
 */
proto.pb.Transaction.Input.prototype.getSignatureScript_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSignatureScript()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.Transaction.Input.prototype.setSignatureScript = function(value) {
  jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional uint32 sequence = 4;
 * @return {number}
 */
proto.pb.Transaction.Input.prototype.getSequence = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/** @param {number} value */
proto.pb.Transaction.Input.prototype.setSequence = function(value) {
  jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 value = 5;
 * @return {number}
 */
proto.pb.Transaction.Input.prototype.getValue = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.Transaction.Input.prototype.setValue = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bytes previous_script = 6;
 * @return {!(string|Uint8Array)}
 */
proto.pb.Transaction.Input.prototype.getPreviousScript = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * optional bytes previous_script = 6;
 * This is a type-conversion wrapper around `getPreviousScript()`
 * @return {string}
 */
proto.pb.Transaction.Input.prototype.getPreviousScript_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPreviousScript()));
};


/**
 * optional bytes previous_script = 6;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPreviousScript()`
 * @return {!Uint8Array}
 */
proto.pb.Transaction.Input.prototype.getPreviousScript_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPreviousScript()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.Transaction.Input.prototype.setPreviousScript = function(value) {
  jspb.Message.setProto3BytesField(this, 6, value);
};


/**
 * optional string address = 7;
 * @return {string}
 */
proto.pb.Transaction.Input.prototype.getAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/** @param {string} value */
proto.pb.Transaction.Input.prototype.setAddress = function(value) {
  jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional SlpToken slp_token = 8;
 * @return {?proto.pb.SlpToken}
 */
proto.pb.Transaction.Input.prototype.getSlpToken = function() {
  return /** @type{?proto.pb.SlpToken} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpToken, 8));
};


/** @param {?proto.pb.SlpToken|undefined} value */
proto.pb.Transaction.Input.prototype.setSlpToken = function(value) {
  jspb.Message.setWrapperField(this, 8, value);
};


proto.pb.Transaction.Input.prototype.clearSlpToken = function() {
  this.setSlpToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Transaction.Input.prototype.hasSlpToken = function() {
  return jspb.Message.getField(this, 8) != null;
};


/**
 * optional CashToken cash_token = 9;
 * @return {?proto.pb.CashToken}
 */
proto.pb.Transaction.Input.prototype.getCashToken = function() {
  return /** @type{?proto.pb.CashToken} */ (
    jspb.Message.getWrapperField(this, proto.pb.CashToken, 9));
};


/** @param {?proto.pb.CashToken|undefined} value */
proto.pb.Transaction.Input.prototype.setCashToken = function(value) {
  jspb.Message.setWrapperField(this, 9, value);
};


proto.pb.Transaction.Input.prototype.clearCashToken = function() {
  this.setCashToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Transaction.Input.prototype.hasCashToken = function() {
  return jspb.Message.getField(this, 9) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.Transaction.Output = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.Transaction.Output, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.Transaction.Output.displayName = 'proto.pb.Transaction.Output';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.Transaction.Output.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.Transaction.Output.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.Transaction.Output} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Transaction.Output.toObject = function(includeInstance, msg) {
  var f, obj = {
    index: jspb.Message.getFieldWithDefault(msg, 1, 0),
    value: jspb.Message.getFieldWithDefault(msg, 2, 0),
    pubkeyScript: msg.getPubkeyScript_asB64(),
    address: jspb.Message.getFieldWithDefault(msg, 4, ""),
    scriptClass: jspb.Message.getFieldWithDefault(msg, 5, ""),
    disassembledScript: jspb.Message.getFieldWithDefault(msg, 6, ""),
    slpToken: (f = msg.getSlpToken()) && proto.pb.SlpToken.toObject(includeInstance, f),
    cashToken: (f = msg.getCashToken()) && proto.pb.CashToken.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.Transaction.Output}
 */
proto.pb.Transaction.Output.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.Transaction.Output;
  return proto.pb.Transaction.Output.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.Transaction.Output} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.Transaction.Output}
 */
proto.pb.Transaction.Output.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setIndex(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setValue(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPubkeyScript(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setAddress(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setScriptClass(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setDisassembledScript(value);
      break;
    case 7:
      var value = new proto.pb.SlpToken;
      reader.readMessage(value,proto.pb.SlpToken.deserializeBinaryFromReader);
      msg.setSlpToken(value);
      break;
    case 8:
      var value = new proto.pb.CashToken;
      reader.readMessage(value,proto.pb.CashToken.deserializeBinaryFromReader);
      msg.setCashToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.Transaction.Output.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.Transaction.Output.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.Transaction.Output} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.Transaction.Output.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIndex();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getValue();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getPubkeyScript_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getAddress();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getScriptClass();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getDisassembledScript();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getSlpToken();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.pb.SlpToken.serializeBinaryToWriter
    );
  }
  f = message.getCashToken();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      proto.pb.CashToken.serializeBinaryToWriter
    );
  }
};


/**
 * optional uint32 index = 1;
 * @return {number}
 */
proto.pb.Transaction.Output.prototype.getIndex = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {number} value */
proto.pb.Transaction.Output.prototype.setIndex = function(value) {
  jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional int64 value = 2;
 * @return {number}
 */
proto.pb.Transaction.Output.prototype.getValue = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.Transaction.Output.prototype.setValue = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional bytes pubkey_script = 3;
 * @return {!(string|Uint8Array)}
 */
proto.pb.Transaction.Output.prototype.getPubkeyScript = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes pubkey_script = 3;
 * This is a type-conversion wrapper around `getPubkeyScript()`
 * @return {string}
 */
proto.pb.Transaction.Output.prototype.getPubkeyScript_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPubkeyScript()));
};


/**
 * optional bytes pubkey_script = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPubkeyScript()`
 * @return {!Uint8Array}
 */
proto.pb.Transaction.Output.prototype.getPubkeyScript_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPubkeyScript()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.Transaction.Output.prototype.setPubkeyScript = function(value) {
  jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional string address = 4;
 * @return {string}
 */
proto.pb.Transaction.Output.prototype.getAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/** @param {string} value */
proto.pb.Transaction.Output.prototype.setAddress = function(value) {
  jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string script_class = 5;
 * @return {string}
 */
proto.pb.Transaction.Output.prototype.getScriptClass = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/** @param {string} value */
proto.pb.Transaction.Output.prototype.setScriptClass = function(value) {
  jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string disassembled_script = 6;
 * @return {string}
 */
proto.pb.Transaction.Output.prototype.getDisassembledScript = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/** @param {string} value */
proto.pb.Transaction.Output.prototype.setDisassembledScript = function(value) {
  jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional SlpToken slp_token = 7;
 * @return {?proto.pb.SlpToken}
 */
proto.pb.Transaction.Output.prototype.getSlpToken = function() {
  return /** @type{?proto.pb.SlpToken} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpToken, 7));
};


/** @param {?proto.pb.SlpToken|undefined} value */
proto.pb.Transaction.Output.prototype.setSlpToken = function(value) {
  jspb.Message.setWrapperField(this, 7, value);
};


proto.pb.Transaction.Output.prototype.clearSlpToken = function() {
  this.setSlpToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Transaction.Output.prototype.hasSlpToken = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * optional CashToken cash_token = 8;
 * @return {?proto.pb.CashToken}
 */
proto.pb.Transaction.Output.prototype.getCashToken = function() {
  return /** @type{?proto.pb.CashToken} */ (
    jspb.Message.getWrapperField(this, proto.pb.CashToken, 8));
};


/** @param {?proto.pb.CashToken|undefined} value */
proto.pb.Transaction.Output.prototype.setCashToken = function(value) {
  jspb.Message.setWrapperField(this, 8, value);
};


proto.pb.Transaction.Output.prototype.clearCashToken = function() {
  this.setCashToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Transaction.Output.prototype.hasCashToken = function() {
  return jspb.Message.getField(this, 8) != null;
};


/**
 * optional bytes hash = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.Transaction.prototype.getHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes hash = 1;
 * This is a type-conversion wrapper around `getHash()`
 * @return {string}
 */
proto.pb.Transaction.prototype.getHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getHash()));
};


/**
 * optional bytes hash = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getHash()`
 * @return {!Uint8Array}
 */
proto.pb.Transaction.prototype.getHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.Transaction.prototype.setHash = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional int32 version = 2;
 * @return {number}
 */
proto.pb.Transaction.prototype.getVersion = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.Transaction.prototype.setVersion = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * repeated Input inputs = 3;
 * @return {!Array<!proto.pb.Transaction.Input>}
 */
proto.pb.Transaction.prototype.getInputsList = function() {
  return /** @type{!Array<!proto.pb.Transaction.Input>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.Transaction.Input, 3));
};


/** @param {!Array<!proto.pb.Transaction.Input>} value */
proto.pb.Transaction.prototype.setInputsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.pb.Transaction.Input=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.Transaction.Input}
 */
proto.pb.Transaction.prototype.addInputs = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.pb.Transaction.Input, opt_index);
};


proto.pb.Transaction.prototype.clearInputsList = function() {
  this.setInputsList([]);
};


/**
 * repeated Output outputs = 4;
 * @return {!Array<!proto.pb.Transaction.Output>}
 */
proto.pb.Transaction.prototype.getOutputsList = function() {
  return /** @type{!Array<!proto.pb.Transaction.Output>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.Transaction.Output, 4));
};


/** @param {!Array<!proto.pb.Transaction.Output>} value */
proto.pb.Transaction.prototype.setOutputsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.pb.Transaction.Output=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.Transaction.Output}
 */
proto.pb.Transaction.prototype.addOutputs = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.pb.Transaction.Output, opt_index);
};


proto.pb.Transaction.prototype.clearOutputsList = function() {
  this.setOutputsList([]);
};


/**
 * optional uint32 lock_time = 5;
 * @return {number}
 */
proto.pb.Transaction.prototype.getLockTime = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.Transaction.prototype.setLockTime = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional int32 size = 8;
 * @return {number}
 */
proto.pb.Transaction.prototype.getSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/** @param {number} value */
proto.pb.Transaction.prototype.setSize = function(value) {
  jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional int64 timestamp = 9;
 * @return {number}
 */
proto.pb.Transaction.prototype.getTimestamp = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/** @param {number} value */
proto.pb.Transaction.prototype.setTimestamp = function(value) {
  jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * optional int32 confirmations = 10;
 * @return {number}
 */
proto.pb.Transaction.prototype.getConfirmations = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 10, 0));
};


/** @param {number} value */
proto.pb.Transaction.prototype.setConfirmations = function(value) {
  jspb.Message.setProto3IntField(this, 10, value);
};


/**
 * optional int32 block_height = 11;
 * @return {number}
 */
proto.pb.Transaction.prototype.getBlockHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 11, 0));
};


/** @param {number} value */
proto.pb.Transaction.prototype.setBlockHeight = function(value) {
  jspb.Message.setProto3IntField(this, 11, value);
};


/**
 * optional bytes block_hash = 12;
 * @return {!(string|Uint8Array)}
 */
proto.pb.Transaction.prototype.getBlockHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * optional bytes block_hash = 12;
 * This is a type-conversion wrapper around `getBlockHash()`
 * @return {string}
 */
proto.pb.Transaction.prototype.getBlockHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getBlockHash()));
};


/**
 * optional bytes block_hash = 12;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getBlockHash()`
 * @return {!Uint8Array}
 */
proto.pb.Transaction.prototype.getBlockHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getBlockHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.Transaction.prototype.setBlockHash = function(value) {
  jspb.Message.setProto3BytesField(this, 12, value);
};


/**
 * optional SlpTransactionInfo slp_transaction_info = 13;
 * @return {?proto.pb.SlpTransactionInfo}
 */
proto.pb.Transaction.prototype.getSlpTransactionInfo = function() {
  return /** @type{?proto.pb.SlpTransactionInfo} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpTransactionInfo, 13));
};


/** @param {?proto.pb.SlpTransactionInfo|undefined} value */
proto.pb.Transaction.prototype.setSlpTransactionInfo = function(value) {
  jspb.Message.setWrapperField(this, 13, value);
};


proto.pb.Transaction.prototype.clearSlpTransactionInfo = function() {
  this.setSlpTransactionInfo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.Transaction.prototype.hasSlpTransactionInfo = function() {
  return jspb.Message.getField(this, 13) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.MempoolTransaction = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.MempoolTransaction, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.MempoolTransaction.displayName = 'proto.pb.MempoolTransaction';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.MempoolTransaction.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.MempoolTransaction.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.MempoolTransaction} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.MempoolTransaction.toObject = function(includeInstance, msg) {
  var f, obj = {
    transaction: (f = msg.getTransaction()) && proto.pb.Transaction.toObject(includeInstance, f),
    addedTime: jspb.Message.getFieldWithDefault(msg, 2, 0),
    addedHeight: jspb.Message.getFieldWithDefault(msg, 3, 0),
    fee: jspb.Message.getFieldWithDefault(msg, 4, 0),
    feePerKb: jspb.Message.getFieldWithDefault(msg, 5, 0),
    startingPriority: +jspb.Message.getFieldWithDefault(msg, 6, 0.0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.MempoolTransaction}
 */
proto.pb.MempoolTransaction.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.MempoolTransaction;
  return proto.pb.MempoolTransaction.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.MempoolTransaction} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.MempoolTransaction}
 */
proto.pb.MempoolTransaction.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.Transaction;
      reader.readMessage(value,proto.pb.Transaction.deserializeBinaryFromReader);
      msg.setTransaction(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setAddedTime(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setAddedHeight(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFee(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFeePerKb(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setStartingPriority(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.MempoolTransaction.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.MempoolTransaction.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.MempoolTransaction} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.MempoolTransaction.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTransaction();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.Transaction.serializeBinaryToWriter
    );
  }
  f = message.getAddedTime();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getAddedHeight();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getFee();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getFeePerKb();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getStartingPriority();
  if (f !== 0.0) {
    writer.writeDouble(
      6,
      f
    );
  }
};


/**
 * optional Transaction transaction = 1;
 * @return {?proto.pb.Transaction}
 */
proto.pb.MempoolTransaction.prototype.getTransaction = function() {
  return /** @type{?proto.pb.Transaction} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction, 1));
};


/** @param {?proto.pb.Transaction|undefined} value */
proto.pb.MempoolTransaction.prototype.setTransaction = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.MempoolTransaction.prototype.clearTransaction = function() {
  this.setTransaction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.MempoolTransaction.prototype.hasTransaction = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 added_time = 2;
 * @return {number}
 */
proto.pb.MempoolTransaction.prototype.getAddedTime = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.pb.MempoolTransaction.prototype.setAddedTime = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int32 added_height = 3;
 * @return {number}
 */
proto.pb.MempoolTransaction.prototype.getAddedHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {number} value */
proto.pb.MempoolTransaction.prototype.setAddedHeight = function(value) {
  jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int64 fee = 4;
 * @return {number}
 */
proto.pb.MempoolTransaction.prototype.getFee = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/** @param {number} value */
proto.pb.MempoolTransaction.prototype.setFee = function(value) {
  jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 fee_per_kb = 5;
 * @return {number}
 */
proto.pb.MempoolTransaction.prototype.getFeePerKb = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.MempoolTransaction.prototype.setFeePerKb = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional double starting_priority = 6;
 * @return {number}
 */
proto.pb.MempoolTransaction.prototype.getStartingPriority = function() {
  return /** @type {number} */ (+jspb.Message.getFieldWithDefault(this, 6, 0.0));
};


/** @param {number} value */
proto.pb.MempoolTransaction.prototype.setStartingPriority = function(value) {
  jspb.Message.setProto3FloatField(this, 6, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.UnspentOutput = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.UnspentOutput, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.UnspentOutput.displayName = 'proto.pb.UnspentOutput';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.UnspentOutput.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.UnspentOutput.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.UnspentOutput} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.UnspentOutput.toObject = function(includeInstance, msg) {
  var f, obj = {
    outpoint: (f = msg.getOutpoint()) && proto.pb.Transaction.Input.Outpoint.toObject(includeInstance, f),
    pubkeyScript: msg.getPubkeyScript_asB64(),
    value: jspb.Message.getFieldWithDefault(msg, 3, 0),
    isCoinbase: jspb.Message.getFieldWithDefault(msg, 4, false),
    blockHeight: jspb.Message.getFieldWithDefault(msg, 5, 0),
    slpToken: (f = msg.getSlpToken()) && proto.pb.SlpToken.toObject(includeInstance, f),
    cashToken: (f = msg.getCashToken()) && proto.pb.CashToken.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.UnspentOutput}
 */
proto.pb.UnspentOutput.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.UnspentOutput;
  return proto.pb.UnspentOutput.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.UnspentOutput} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.UnspentOutput}
 */
proto.pb.UnspentOutput.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.Transaction.Input.Outpoint;
      reader.readMessage(value,proto.pb.Transaction.Input.Outpoint.deserializeBinaryFromReader);
      msg.setOutpoint(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPubkeyScript(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setValue(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIsCoinbase(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setBlockHeight(value);
      break;
    case 6:
      var value = new proto.pb.SlpToken;
      reader.readMessage(value,proto.pb.SlpToken.deserializeBinaryFromReader);
      msg.setSlpToken(value);
      break;
    case 7:
      var value = new proto.pb.CashToken;
      reader.readMessage(value,proto.pb.CashToken.deserializeBinaryFromReader);
      msg.setCashToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.UnspentOutput.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.UnspentOutput.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.UnspentOutput} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.UnspentOutput.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOutpoint();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.Transaction.Input.Outpoint.serializeBinaryToWriter
    );
  }
  f = message.getPubkeyScript_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
  f = message.getValue();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
  f = message.getIsCoinbase();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getBlockHeight();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
  f = message.getSlpToken();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      proto.pb.SlpToken.serializeBinaryToWriter
    );
  }
  f = message.getCashToken();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.pb.CashToken.serializeBinaryToWriter
    );
  }
};


/**
 * optional Transaction.Input.Outpoint outpoint = 1;
 * @return {?proto.pb.Transaction.Input.Outpoint}
 */
proto.pb.UnspentOutput.prototype.getOutpoint = function() {
  return /** @type{?proto.pb.Transaction.Input.Outpoint} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction.Input.Outpoint, 1));
};


/** @param {?proto.pb.Transaction.Input.Outpoint|undefined} value */
proto.pb.UnspentOutput.prototype.setOutpoint = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.UnspentOutput.prototype.clearOutpoint = function() {
  this.setOutpoint(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.UnspentOutput.prototype.hasOutpoint = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bytes pubkey_script = 2;
 * @return {!(string|Uint8Array)}
 */
proto.pb.UnspentOutput.prototype.getPubkeyScript = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes pubkey_script = 2;
 * This is a type-conversion wrapper around `getPubkeyScript()`
 * @return {string}
 */
proto.pb.UnspentOutput.prototype.getPubkeyScript_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPubkeyScript()));
};


/**
 * optional bytes pubkey_script = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPubkeyScript()`
 * @return {!Uint8Array}
 */
proto.pb.UnspentOutput.prototype.getPubkeyScript_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPubkeyScript()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.UnspentOutput.prototype.setPubkeyScript = function(value) {
  jspb.Message.setProto3BytesField(this, 2, value);
};


/**
 * optional int64 value = 3;
 * @return {number}
 */
proto.pb.UnspentOutput.prototype.getValue = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {number} value */
proto.pb.UnspentOutput.prototype.setValue = function(value) {
  jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional bool is_coinbase = 4;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.UnspentOutput.prototype.getIsCoinbase = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 4, false));
};


/** @param {boolean} value */
proto.pb.UnspentOutput.prototype.setIsCoinbase = function(value) {
  jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional int32 block_height = 5;
 * @return {number}
 */
proto.pb.UnspentOutput.prototype.getBlockHeight = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.UnspentOutput.prototype.setBlockHeight = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional SlpToken slp_token = 6;
 * @return {?proto.pb.SlpToken}
 */
proto.pb.UnspentOutput.prototype.getSlpToken = function() {
  return /** @type{?proto.pb.SlpToken} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpToken, 6));
};


/** @param {?proto.pb.SlpToken|undefined} value */
proto.pb.UnspentOutput.prototype.setSlpToken = function(value) {
  jspb.Message.setWrapperField(this, 6, value);
};


proto.pb.UnspentOutput.prototype.clearSlpToken = function() {
  this.setSlpToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.UnspentOutput.prototype.hasSlpToken = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional CashToken cash_token = 7;
 * @return {?proto.pb.CashToken}
 */
proto.pb.UnspentOutput.prototype.getCashToken = function() {
  return /** @type{?proto.pb.CashToken} */ (
    jspb.Message.getWrapperField(this, proto.pb.CashToken, 7));
};


/** @param {?proto.pb.CashToken|undefined} value */
proto.pb.UnspentOutput.prototype.setCashToken = function(value) {
  jspb.Message.setWrapperField(this, 7, value);
};


proto.pb.UnspentOutput.prototype.clearCashToken = function() {
  this.setCashToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.UnspentOutput.prototype.hasCashToken = function() {
  return jspb.Message.getField(this, 7) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.TransactionFilter = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.TransactionFilter.repeatedFields_, null);
};
goog.inherits(proto.pb.TransactionFilter, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.TransactionFilter.displayName = 'proto.pb.TransactionFilter';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.TransactionFilter.repeatedFields_ = [1,2,3,6];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.TransactionFilter.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.TransactionFilter.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.TransactionFilter} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.TransactionFilter.toObject = function(includeInstance, msg) {
  var f, obj = {
    addressesList: jspb.Message.getRepeatedField(msg, 1),
    outpointsList: jspb.Message.toObjectList(msg.getOutpointsList(),
    proto.pb.Transaction.Input.Outpoint.toObject, includeInstance),
    dataElementsList: msg.getDataElementsList_asB64(),
    allTransactions: jspb.Message.getFieldWithDefault(msg, 4, false),
    allSlpTransactions: jspb.Message.getFieldWithDefault(msg, 5, false),
    slpTokenIdsList: msg.getSlpTokenIdsList_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.TransactionFilter}
 */
proto.pb.TransactionFilter.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.TransactionFilter;
  return proto.pb.TransactionFilter.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.TransactionFilter} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.TransactionFilter}
 */
proto.pb.TransactionFilter.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addAddresses(value);
      break;
    case 2:
      var value = new proto.pb.Transaction.Input.Outpoint;
      reader.readMessage(value,proto.pb.Transaction.Input.Outpoint.deserializeBinaryFromReader);
      msg.addOutpoints(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addDataElements(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllTransactions(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllSlpTransactions(value);
      break;
    case 6:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.addSlpTokenIds(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.TransactionFilter.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.TransactionFilter.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.TransactionFilter} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.TransactionFilter.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAddressesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
  f = message.getOutpointsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.pb.Transaction.Input.Outpoint.serializeBinaryToWriter
    );
  }
  f = message.getDataElementsList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      3,
      f
    );
  }
  f = message.getAllTransactions();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getAllSlpTransactions();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getSlpTokenIdsList_asU8();
  if (f.length > 0) {
    writer.writeRepeatedBytes(
      6,
      f
    );
  }
};


/**
 * repeated string addresses = 1;
 * @return {!Array<string>}
 */
proto.pb.TransactionFilter.prototype.getAddressesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/** @param {!Array<string>} value */
proto.pb.TransactionFilter.prototype.setAddressesList = function(value) {
  jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {!string} value
 * @param {number=} opt_index
 */
proto.pb.TransactionFilter.prototype.addAddresses = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


proto.pb.TransactionFilter.prototype.clearAddressesList = function() {
  this.setAddressesList([]);
};


/**
 * repeated Transaction.Input.Outpoint outpoints = 2;
 * @return {!Array<!proto.pb.Transaction.Input.Outpoint>}
 */
proto.pb.TransactionFilter.prototype.getOutpointsList = function() {
  return /** @type{!Array<!proto.pb.Transaction.Input.Outpoint>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.pb.Transaction.Input.Outpoint, 2));
};


/** @param {!Array<!proto.pb.Transaction.Input.Outpoint>} value */
proto.pb.TransactionFilter.prototype.setOutpointsList = function(value) {
  jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.pb.Transaction.Input.Outpoint=} opt_value
 * @param {number=} opt_index
 * @return {!proto.pb.Transaction.Input.Outpoint}
 */
proto.pb.TransactionFilter.prototype.addOutpoints = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.pb.Transaction.Input.Outpoint, opt_index);
};


proto.pb.TransactionFilter.prototype.clearOutpointsList = function() {
  this.setOutpointsList([]);
};


/**
 * repeated bytes data_elements = 3;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.TransactionFilter.prototype.getDataElementsList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * repeated bytes data_elements = 3;
 * This is a type-conversion wrapper around `getDataElementsList()`
 * @return {!Array<string>}
 */
proto.pb.TransactionFilter.prototype.getDataElementsList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getDataElementsList()));
};


/**
 * repeated bytes data_elements = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getDataElementsList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.TransactionFilter.prototype.getDataElementsList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getDataElementsList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.TransactionFilter.prototype.setDataElementsList = function(value) {
  jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.TransactionFilter.prototype.addDataElements = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


proto.pb.TransactionFilter.prototype.clearDataElementsList = function() {
  this.setDataElementsList([]);
};


/**
 * optional bool all_transactions = 4;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.TransactionFilter.prototype.getAllTransactions = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 4, false));
};


/** @param {boolean} value */
proto.pb.TransactionFilter.prototype.setAllTransactions = function(value) {
  jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional bool all_slp_transactions = 5;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.TransactionFilter.prototype.getAllSlpTransactions = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 5, false));
};


/** @param {boolean} value */
proto.pb.TransactionFilter.prototype.setAllSlpTransactions = function(value) {
  jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * repeated bytes slp_token_ids = 6;
 * @return {!(Array<!Uint8Array>|Array<string>)}
 */
proto.pb.TransactionFilter.prototype.getSlpTokenIdsList = function() {
  return /** @type {!(Array<!Uint8Array>|Array<string>)} */ (jspb.Message.getRepeatedField(this, 6));
};


/**
 * repeated bytes slp_token_ids = 6;
 * This is a type-conversion wrapper around `getSlpTokenIdsList()`
 * @return {!Array<string>}
 */
proto.pb.TransactionFilter.prototype.getSlpTokenIdsList_asB64 = function() {
  return /** @type {!Array<string>} */ (jspb.Message.bytesListAsB64(
      this.getSlpTokenIdsList()));
};


/**
 * repeated bytes slp_token_ids = 6;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSlpTokenIdsList()`
 * @return {!Array<!Uint8Array>}
 */
proto.pb.TransactionFilter.prototype.getSlpTokenIdsList_asU8 = function() {
  return /** @type {!Array<!Uint8Array>} */ (jspb.Message.bytesListAsU8(
      this.getSlpTokenIdsList()));
};


/** @param {!(Array<!Uint8Array>|Array<string>)} value */
proto.pb.TransactionFilter.prototype.setSlpTokenIdsList = function(value) {
  jspb.Message.setField(this, 6, value || []);
};


/**
 * @param {!(string|Uint8Array)} value
 * @param {number=} opt_index
 */
proto.pb.TransactionFilter.prototype.addSlpTokenIds = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 6, value, opt_index);
};


proto.pb.TransactionFilter.prototype.clearSlpTokenIdsList = function() {
  this.setSlpTokenIdsList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.CashToken = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.CashToken, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.CashToken.displayName = 'proto.pb.CashToken';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.CashToken.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.CashToken.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.CashToken} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.CashToken.toObject = function(includeInstance, msg) {
  var f, obj = {
    categoryId: msg.getCategoryId_asB64(),
    amount: jspb.Message.getFieldWithDefault(msg, 2, "0"),
    commitment: msg.getCommitment_asB64(),
    bitfield: msg.getBitfield_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.CashToken}
 */
proto.pb.CashToken.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.CashToken;
  return proto.pb.CashToken.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.CashToken} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.CashToken}
 */
proto.pb.CashToken.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setCategoryId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readUint64String());
      msg.setAmount(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setCommitment(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setBitfield(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.CashToken.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.CashToken.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.CashToken} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.CashToken.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCategoryId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getAmount();
  if (parseInt(f, 10) !== 0) {
    writer.writeUint64String(
      2,
      f
    );
  }
  f = message.getCommitment_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getBitfield_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
};


/**
 * optional bytes category_id = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.CashToken.prototype.getCategoryId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes category_id = 1;
 * This is a type-conversion wrapper around `getCategoryId()`
 * @return {string}
 */
proto.pb.CashToken.prototype.getCategoryId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getCategoryId()));
};


/**
 * optional bytes category_id = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getCategoryId()`
 * @return {!Uint8Array}
 */
proto.pb.CashToken.prototype.getCategoryId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getCategoryId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.CashToken.prototype.setCategoryId = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional uint64 amount = 2;
 * @return {string}
 */
proto.pb.CashToken.prototype.getAmount = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, "0"));
};


/** @param {string} value */
proto.pb.CashToken.prototype.setAmount = function(value) {
  jspb.Message.setProto3StringIntField(this, 2, value);
};


/**
 * optional bytes commitment = 3;
 * @return {!(string|Uint8Array)}
 */
proto.pb.CashToken.prototype.getCommitment = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes commitment = 3;
 * This is a type-conversion wrapper around `getCommitment()`
 * @return {string}
 */
proto.pb.CashToken.prototype.getCommitment_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getCommitment()));
};


/**
 * optional bytes commitment = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getCommitment()`
 * @return {!Uint8Array}
 */
proto.pb.CashToken.prototype.getCommitment_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getCommitment()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.CashToken.prototype.setCommitment = function(value) {
  jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional bytes bitfield = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.CashToken.prototype.getBitfield = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes bitfield = 4;
 * This is a type-conversion wrapper around `getBitfield()`
 * @return {string}
 */
proto.pb.CashToken.prototype.getBitfield_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getBitfield()));
};


/**
 * optional bytes bitfield = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getBitfield()`
 * @return {!Uint8Array}
 */
proto.pb.CashToken.prototype.getBitfield_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getBitfield()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.CashToken.prototype.setBitfield = function(value) {
  jspb.Message.setProto3BytesField(this, 4, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpToken = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SlpToken, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpToken.displayName = 'proto.pb.SlpToken';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpToken.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpToken.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpToken} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpToken.toObject = function(includeInstance, msg) {
  var f, obj = {
    tokenId: msg.getTokenId_asB64(),
    amount: jspb.Message.getFieldWithDefault(msg, 2, "0"),
    isMintBaton: jspb.Message.getFieldWithDefault(msg, 3, false),
    address: jspb.Message.getFieldWithDefault(msg, 4, ""),
    decimals: jspb.Message.getFieldWithDefault(msg, 5, 0),
    slpAction: jspb.Message.getFieldWithDefault(msg, 6, 0),
    tokenType: jspb.Message.getFieldWithDefault(msg, 7, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpToken}
 */
proto.pb.SlpToken.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpToken;
  return proto.pb.SlpToken.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpToken} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpToken}
 */
proto.pb.SlpToken.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readUint64String());
      msg.setAmount(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIsMintBaton(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setAddress(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setDecimals(value);
      break;
    case 6:
      var value = /** @type {!proto.pb.SlpAction} */ (reader.readEnum());
      msg.setSlpAction(value);
      break;
    case 7:
      var value = /** @type {!proto.pb.SlpTokenType} */ (reader.readEnum());
      msg.setTokenType(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpToken.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpToken.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpToken} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpToken.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTokenId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getAmount();
  if (parseInt(f, 10) !== 0) {
    writer.writeUint64String(
      2,
      f
    );
  }
  f = message.getIsMintBaton();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getAddress();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getDecimals();
  if (f !== 0) {
    writer.writeUint32(
      5,
      f
    );
  }
  f = message.getSlpAction();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
  f = message.getTokenType();
  if (f !== 0.0) {
    writer.writeEnum(
      7,
      f
    );
  }
};


/**
 * optional bytes token_id = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpToken.prototype.getTokenId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes token_id = 1;
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {string}
 */
proto.pb.SlpToken.prototype.getTokenId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenId()));
};


/**
 * optional bytes token_id = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {!Uint8Array}
 */
proto.pb.SlpToken.prototype.getTokenId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpToken.prototype.setTokenId = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional uint64 amount = 2;
 * @return {string}
 */
proto.pb.SlpToken.prototype.getAmount = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, "0"));
};


/** @param {string} value */
proto.pb.SlpToken.prototype.setAmount = function(value) {
  jspb.Message.setProto3StringIntField(this, 2, value);
};


/**
 * optional bool is_mint_baton = 3;
 * Note that Boolean fields may be set to 0/1 when serialized from a Java server.
 * You should avoid comparisons like {@code val === true/false} in those cases.
 * @return {boolean}
 */
proto.pb.SlpToken.prototype.getIsMintBaton = function() {
  return /** @type {boolean} */ (jspb.Message.getFieldWithDefault(this, 3, false));
};


/** @param {boolean} value */
proto.pb.SlpToken.prototype.setIsMintBaton = function(value) {
  jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional string address = 4;
 * @return {string}
 */
proto.pb.SlpToken.prototype.getAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/** @param {string} value */
proto.pb.SlpToken.prototype.setAddress = function(value) {
  jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional uint32 decimals = 5;
 * @return {number}
 */
proto.pb.SlpToken.prototype.getDecimals = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.SlpToken.prototype.setDecimals = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional SlpAction slp_action = 6;
 * @return {!proto.pb.SlpAction}
 */
proto.pb.SlpToken.prototype.getSlpAction = function() {
  return /** @type {!proto.pb.SlpAction} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/** @param {!proto.pb.SlpAction} value */
proto.pb.SlpToken.prototype.setSlpAction = function(value) {
  jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional SlpTokenType token_type = 7;
 * @return {!proto.pb.SlpTokenType}
 */
proto.pb.SlpToken.prototype.getTokenType = function() {
  return /** @type {!proto.pb.SlpTokenType} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/** @param {!proto.pb.SlpTokenType} value */
proto.pb.SlpToken.prototype.setTokenType = function(value) {
  jspb.Message.setProto3EnumField(this, 7, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpTransactionInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.SlpTransactionInfo.repeatedFields_, proto.pb.SlpTransactionInfo.oneofGroups_);
};
goog.inherits(proto.pb.SlpTransactionInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpTransactionInfo.displayName = 'proto.pb.SlpTransactionInfo';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.SlpTransactionInfo.repeatedFields_ = [5];

/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.SlpTransactionInfo.oneofGroups_ = [[6,7,8,9,10]];

/**
 * @enum {number}
 */
proto.pb.SlpTransactionInfo.TxMetadataCase = {
  TX_METADATA_NOT_SET: 0,
  V1_GENESIS: 6,
  V1_MINT: 7,
  V1_SEND: 8,
  V1_NFT1_CHILD_GENESIS: 9,
  V1_NFT1_CHILD_SEND: 10
};

/**
 * @return {proto.pb.SlpTransactionInfo.TxMetadataCase}
 */
proto.pb.SlpTransactionInfo.prototype.getTxMetadataCase = function() {
  return /** @type {proto.pb.SlpTransactionInfo.TxMetadataCase} */(jspb.Message.computeOneofCase(this, proto.pb.SlpTransactionInfo.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpTransactionInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpTransactionInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpTransactionInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTransactionInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
    slpAction: jspb.Message.getFieldWithDefault(msg, 1, 0),
    validityJudgement: jspb.Message.getFieldWithDefault(msg, 2, 0),
    parseError: jspb.Message.getFieldWithDefault(msg, 3, ""),
    tokenId: msg.getTokenId_asB64(),
    burnFlagsList: jspb.Message.getRepeatedField(msg, 5),
    v1Genesis: (f = msg.getV1Genesis()) && proto.pb.SlpV1GenesisMetadata.toObject(includeInstance, f),
    v1Mint: (f = msg.getV1Mint()) && proto.pb.SlpV1MintMetadata.toObject(includeInstance, f),
    v1Send: (f = msg.getV1Send()) && proto.pb.SlpV1SendMetadata.toObject(includeInstance, f),
    v1Nft1ChildGenesis: (f = msg.getV1Nft1ChildGenesis()) && proto.pb.SlpV1Nft1ChildGenesisMetadata.toObject(includeInstance, f),
    v1Nft1ChildSend: (f = msg.getV1Nft1ChildSend()) && proto.pb.SlpV1Nft1ChildSendMetadata.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpTransactionInfo}
 */
proto.pb.SlpTransactionInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpTransactionInfo;
  return proto.pb.SlpTransactionInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpTransactionInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpTransactionInfo}
 */
proto.pb.SlpTransactionInfo.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.pb.SlpAction} */ (reader.readEnum());
      msg.setSlpAction(value);
      break;
    case 2:
      var value = /** @type {!proto.pb.SlpTransactionInfo.ValidityJudgement} */ (reader.readEnum());
      msg.setValidityJudgement(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setParseError(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenId(value);
      break;
    case 5:
      var value = /** @type {!Array<!proto.pb.SlpTransactionInfo.BurnFlags>} */ (reader.readPackedEnum());
      msg.setBurnFlagsList(value);
      break;
    case 6:
      var value = new proto.pb.SlpV1GenesisMetadata;
      reader.readMessage(value,proto.pb.SlpV1GenesisMetadata.deserializeBinaryFromReader);
      msg.setV1Genesis(value);
      break;
    case 7:
      var value = new proto.pb.SlpV1MintMetadata;
      reader.readMessage(value,proto.pb.SlpV1MintMetadata.deserializeBinaryFromReader);
      msg.setV1Mint(value);
      break;
    case 8:
      var value = new proto.pb.SlpV1SendMetadata;
      reader.readMessage(value,proto.pb.SlpV1SendMetadata.deserializeBinaryFromReader);
      msg.setV1Send(value);
      break;
    case 9:
      var value = new proto.pb.SlpV1Nft1ChildGenesisMetadata;
      reader.readMessage(value,proto.pb.SlpV1Nft1ChildGenesisMetadata.deserializeBinaryFromReader);
      msg.setV1Nft1ChildGenesis(value);
      break;
    case 10:
      var value = new proto.pb.SlpV1Nft1ChildSendMetadata;
      reader.readMessage(value,proto.pb.SlpV1Nft1ChildSendMetadata.deserializeBinaryFromReader);
      msg.setV1Nft1ChildSend(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpTransactionInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpTransactionInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpTransactionInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTransactionInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSlpAction();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getValidityJudgement();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getParseError();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getTokenId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getBurnFlagsList();
  if (f.length > 0) {
    writer.writePackedEnum(
      5,
      f
    );
  }
  f = message.getV1Genesis();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      proto.pb.SlpV1GenesisMetadata.serializeBinaryToWriter
    );
  }
  f = message.getV1Mint();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.pb.SlpV1MintMetadata.serializeBinaryToWriter
    );
  }
  f = message.getV1Send();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      proto.pb.SlpV1SendMetadata.serializeBinaryToWriter
    );
  }
  f = message.getV1Nft1ChildGenesis();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      proto.pb.SlpV1Nft1ChildGenesisMetadata.serializeBinaryToWriter
    );
  }
  f = message.getV1Nft1ChildSend();
  if (f != null) {
    writer.writeMessage(
      10,
      f,
      proto.pb.SlpV1Nft1ChildSendMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * @enum {number}
 */
proto.pb.SlpTransactionInfo.ValidityJudgement = {
  UNKNOWN_OR_INVALID: 0,
  VALID: 1
};

/**
 * @enum {number}
 */
proto.pb.SlpTransactionInfo.BurnFlags = {
  BURNED_INPUTS_OUTPUTS_TOO_HIGH: 0,
  BURNED_INPUTS_BAD_OPRETURN: 1,
  BURNED_INPUTS_OTHER_TOKEN: 2,
  BURNED_OUTPUTS_MISSING_BCH_VOUT: 3,
  BURNED_INPUTS_GREATER_THAN_OUTPUTS: 4
};

/**
 * optional SlpAction slp_action = 1;
 * @return {!proto.pb.SlpAction}
 */
proto.pb.SlpTransactionInfo.prototype.getSlpAction = function() {
  return /** @type {!proto.pb.SlpAction} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {!proto.pb.SlpAction} value */
proto.pb.SlpTransactionInfo.prototype.setSlpAction = function(value) {
  jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional ValidityJudgement validity_judgement = 2;
 * @return {!proto.pb.SlpTransactionInfo.ValidityJudgement}
 */
proto.pb.SlpTransactionInfo.prototype.getValidityJudgement = function() {
  return /** @type {!proto.pb.SlpTransactionInfo.ValidityJudgement} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {!proto.pb.SlpTransactionInfo.ValidityJudgement} value */
proto.pb.SlpTransactionInfo.prototype.setValidityJudgement = function(value) {
  jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string parse_error = 3;
 * @return {string}
 */
proto.pb.SlpTransactionInfo.prototype.getParseError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/** @param {string} value */
proto.pb.SlpTransactionInfo.prototype.setParseError = function(value) {
  jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bytes token_id = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpTransactionInfo.prototype.getTokenId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes token_id = 4;
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {string}
 */
proto.pb.SlpTransactionInfo.prototype.getTokenId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenId()));
};


/**
 * optional bytes token_id = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {!Uint8Array}
 */
proto.pb.SlpTransactionInfo.prototype.getTokenId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpTransactionInfo.prototype.setTokenId = function(value) {
  jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * repeated BurnFlags burn_flags = 5;
 * @return {!Array<!proto.pb.SlpTransactionInfo.BurnFlags>}
 */
proto.pb.SlpTransactionInfo.prototype.getBurnFlagsList = function() {
  return /** @type {!Array<!proto.pb.SlpTransactionInfo.BurnFlags>} */ (jspb.Message.getRepeatedField(this, 5));
};


/** @param {!Array<!proto.pb.SlpTransactionInfo.BurnFlags>} value */
proto.pb.SlpTransactionInfo.prototype.setBurnFlagsList = function(value) {
  jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {!proto.pb.SlpTransactionInfo.BurnFlags} value
 * @param {number=} opt_index
 */
proto.pb.SlpTransactionInfo.prototype.addBurnFlags = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


proto.pb.SlpTransactionInfo.prototype.clearBurnFlagsList = function() {
  this.setBurnFlagsList([]);
};


/**
 * optional SlpV1GenesisMetadata v1_genesis = 6;
 * @return {?proto.pb.SlpV1GenesisMetadata}
 */
proto.pb.SlpTransactionInfo.prototype.getV1Genesis = function() {
  return /** @type{?proto.pb.SlpV1GenesisMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1GenesisMetadata, 6));
};


/** @param {?proto.pb.SlpV1GenesisMetadata|undefined} value */
proto.pb.SlpTransactionInfo.prototype.setV1Genesis = function(value) {
  jspb.Message.setOneofWrapperField(this, 6, proto.pb.SlpTransactionInfo.oneofGroups_[0], value);
};


proto.pb.SlpTransactionInfo.prototype.clearV1Genesis = function() {
  this.setV1Genesis(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpTransactionInfo.prototype.hasV1Genesis = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional SlpV1MintMetadata v1_mint = 7;
 * @return {?proto.pb.SlpV1MintMetadata}
 */
proto.pb.SlpTransactionInfo.prototype.getV1Mint = function() {
  return /** @type{?proto.pb.SlpV1MintMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1MintMetadata, 7));
};


/** @param {?proto.pb.SlpV1MintMetadata|undefined} value */
proto.pb.SlpTransactionInfo.prototype.setV1Mint = function(value) {
  jspb.Message.setOneofWrapperField(this, 7, proto.pb.SlpTransactionInfo.oneofGroups_[0], value);
};


proto.pb.SlpTransactionInfo.prototype.clearV1Mint = function() {
  this.setV1Mint(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpTransactionInfo.prototype.hasV1Mint = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * optional SlpV1SendMetadata v1_send = 8;
 * @return {?proto.pb.SlpV1SendMetadata}
 */
proto.pb.SlpTransactionInfo.prototype.getV1Send = function() {
  return /** @type{?proto.pb.SlpV1SendMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1SendMetadata, 8));
};


/** @param {?proto.pb.SlpV1SendMetadata|undefined} value */
proto.pb.SlpTransactionInfo.prototype.setV1Send = function(value) {
  jspb.Message.setOneofWrapperField(this, 8, proto.pb.SlpTransactionInfo.oneofGroups_[0], value);
};


proto.pb.SlpTransactionInfo.prototype.clearV1Send = function() {
  this.setV1Send(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpTransactionInfo.prototype.hasV1Send = function() {
  return jspb.Message.getField(this, 8) != null;
};


/**
 * optional SlpV1Nft1ChildGenesisMetadata v1_nft1_child_genesis = 9;
 * @return {?proto.pb.SlpV1Nft1ChildGenesisMetadata}
 */
proto.pb.SlpTransactionInfo.prototype.getV1Nft1ChildGenesis = function() {
  return /** @type{?proto.pb.SlpV1Nft1ChildGenesisMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1Nft1ChildGenesisMetadata, 9));
};


/** @param {?proto.pb.SlpV1Nft1ChildGenesisMetadata|undefined} value */
proto.pb.SlpTransactionInfo.prototype.setV1Nft1ChildGenesis = function(value) {
  jspb.Message.setOneofWrapperField(this, 9, proto.pb.SlpTransactionInfo.oneofGroups_[0], value);
};


proto.pb.SlpTransactionInfo.prototype.clearV1Nft1ChildGenesis = function() {
  this.setV1Nft1ChildGenesis(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpTransactionInfo.prototype.hasV1Nft1ChildGenesis = function() {
  return jspb.Message.getField(this, 9) != null;
};


/**
 * optional SlpV1Nft1ChildSendMetadata v1_nft1_child_send = 10;
 * @return {?proto.pb.SlpV1Nft1ChildSendMetadata}
 */
proto.pb.SlpTransactionInfo.prototype.getV1Nft1ChildSend = function() {
  return /** @type{?proto.pb.SlpV1Nft1ChildSendMetadata} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpV1Nft1ChildSendMetadata, 10));
};


/** @param {?proto.pb.SlpV1Nft1ChildSendMetadata|undefined} value */
proto.pb.SlpTransactionInfo.prototype.setV1Nft1ChildSend = function(value) {
  jspb.Message.setOneofWrapperField(this, 10, proto.pb.SlpTransactionInfo.oneofGroups_[0], value);
};


proto.pb.SlpTransactionInfo.prototype.clearV1Nft1ChildSend = function() {
  this.setV1Nft1ChildSend(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpTransactionInfo.prototype.hasV1Nft1ChildSend = function() {
  return jspb.Message.getField(this, 10) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpV1GenesisMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SlpV1GenesisMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpV1GenesisMetadata.displayName = 'proto.pb.SlpV1GenesisMetadata';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpV1GenesisMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpV1GenesisMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpV1GenesisMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1GenesisMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: msg.getName_asB64(),
    ticker: msg.getTicker_asB64(),
    documentUrl: msg.getDocumentUrl_asB64(),
    documentHash: msg.getDocumentHash_asB64(),
    decimals: jspb.Message.getFieldWithDefault(msg, 5, 0),
    mintBatonVout: jspb.Message.getFieldWithDefault(msg, 6, 0),
    mintAmount: jspb.Message.getFieldWithDefault(msg, 7, "0")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpV1GenesisMetadata}
 */
proto.pb.SlpV1GenesisMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpV1GenesisMetadata;
  return proto.pb.SlpV1GenesisMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpV1GenesisMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpV1GenesisMetadata}
 */
proto.pb.SlpV1GenesisMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setName(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTicker(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setDocumentUrl(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setDocumentHash(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setDecimals(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setMintBatonVout(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readUint64String());
      msg.setMintAmount(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpV1GenesisMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpV1GenesisMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpV1GenesisMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1GenesisMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getTicker_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
  f = message.getDocumentUrl_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getDocumentHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getDecimals();
  if (f !== 0) {
    writer.writeUint32(
      5,
      f
    );
  }
  f = message.getMintBatonVout();
  if (f !== 0) {
    writer.writeUint32(
      6,
      f
    );
  }
  f = message.getMintAmount();
  if (parseInt(f, 10) !== 0) {
    writer.writeUint64String(
      7,
      f
    );
  }
};


/**
 * optional bytes name = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getName = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes name = 1;
 * This is a type-conversion wrapper around `getName()`
 * @return {string}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getName_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getName()));
};


/**
 * optional bytes name = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getName()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getName_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getName()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1GenesisMetadata.prototype.setName = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional bytes ticker = 2;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getTicker = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes ticker = 2;
 * This is a type-conversion wrapper around `getTicker()`
 * @return {string}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getTicker_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTicker()));
};


/**
 * optional bytes ticker = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTicker()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getTicker_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTicker()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1GenesisMetadata.prototype.setTicker = function(value) {
  jspb.Message.setProto3BytesField(this, 2, value);
};


/**
 * optional bytes document_url = 3;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getDocumentUrl = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes document_url = 3;
 * This is a type-conversion wrapper around `getDocumentUrl()`
 * @return {string}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getDocumentUrl_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getDocumentUrl()));
};


/**
 * optional bytes document_url = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getDocumentUrl()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getDocumentUrl_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getDocumentUrl()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1GenesisMetadata.prototype.setDocumentUrl = function(value) {
  jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional bytes document_hash = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getDocumentHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes document_hash = 4;
 * This is a type-conversion wrapper around `getDocumentHash()`
 * @return {string}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getDocumentHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getDocumentHash()));
};


/**
 * optional bytes document_hash = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getDocumentHash()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getDocumentHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getDocumentHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1GenesisMetadata.prototype.setDocumentHash = function(value) {
  jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * optional uint32 decimals = 5;
 * @return {number}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getDecimals = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.SlpV1GenesisMetadata.prototype.setDecimals = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional uint32 mint_baton_vout = 6;
 * @return {number}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getMintBatonVout = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/** @param {number} value */
proto.pb.SlpV1GenesisMetadata.prototype.setMintBatonVout = function(value) {
  jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional uint64 mint_amount = 7;
 * @return {string}
 */
proto.pb.SlpV1GenesisMetadata.prototype.getMintAmount = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, "0"));
};


/** @param {string} value */
proto.pb.SlpV1GenesisMetadata.prototype.setMintAmount = function(value) {
  jspb.Message.setProto3StringIntField(this, 7, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpV1MintMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SlpV1MintMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpV1MintMetadata.displayName = 'proto.pb.SlpV1MintMetadata';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpV1MintMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpV1MintMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpV1MintMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1MintMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    mintBatonVout: jspb.Message.getFieldWithDefault(msg, 1, 0),
    mintAmount: jspb.Message.getFieldWithDefault(msg, 2, "0")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpV1MintMetadata}
 */
proto.pb.SlpV1MintMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpV1MintMetadata;
  return proto.pb.SlpV1MintMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpV1MintMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpV1MintMetadata}
 */
proto.pb.SlpV1MintMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setMintBatonVout(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readUint64String());
      msg.setMintAmount(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpV1MintMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpV1MintMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpV1MintMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1MintMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMintBatonVout();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getMintAmount();
  if (parseInt(f, 10) !== 0) {
    writer.writeUint64String(
      2,
      f
    );
  }
};


/**
 * optional uint32 mint_baton_vout = 1;
 * @return {number}
 */
proto.pb.SlpV1MintMetadata.prototype.getMintBatonVout = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {number} value */
proto.pb.SlpV1MintMetadata.prototype.setMintBatonVout = function(value) {
  jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional uint64 mint_amount = 2;
 * @return {string}
 */
proto.pb.SlpV1MintMetadata.prototype.getMintAmount = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, "0"));
};


/** @param {string} value */
proto.pb.SlpV1MintMetadata.prototype.setMintAmount = function(value) {
  jspb.Message.setProto3StringIntField(this, 2, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpV1SendMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.pb.SlpV1SendMetadata.repeatedFields_, null);
};
goog.inherits(proto.pb.SlpV1SendMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpV1SendMetadata.displayName = 'proto.pb.SlpV1SendMetadata';
}
/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.pb.SlpV1SendMetadata.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpV1SendMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpV1SendMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpV1SendMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1SendMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    amountsList: jspb.Message.getRepeatedField(msg, 1)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpV1SendMetadata}
 */
proto.pb.SlpV1SendMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpV1SendMetadata;
  return proto.pb.SlpV1SendMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpV1SendMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpV1SendMetadata}
 */
proto.pb.SlpV1SendMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Array<string>} */ (reader.readPackedUint64String());
      msg.setAmountsList(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpV1SendMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpV1SendMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpV1SendMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1SendMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAmountsList();
  if (f.length > 0) {
    writer.writePackedUint64String(
      1,
      f
    );
  }
};


/**
 * repeated uint64 amounts = 1;
 * @return {!Array<string>}
 */
proto.pb.SlpV1SendMetadata.prototype.getAmountsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/** @param {!Array<string>} value */
proto.pb.SlpV1SendMetadata.prototype.setAmountsList = function(value) {
  jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {!string} value
 * @param {number=} opt_index
 */
proto.pb.SlpV1SendMetadata.prototype.addAmounts = function(value, opt_index) {
  jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


proto.pb.SlpV1SendMetadata.prototype.clearAmountsList = function() {
  this.setAmountsList([]);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SlpV1Nft1ChildGenesisMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpV1Nft1ChildGenesisMetadata.displayName = 'proto.pb.SlpV1Nft1ChildGenesisMetadata';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpV1Nft1ChildGenesisMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpV1Nft1ChildGenesisMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: msg.getName_asB64(),
    ticker: msg.getTicker_asB64(),
    documentUrl: msg.getDocumentUrl_asB64(),
    documentHash: msg.getDocumentHash_asB64(),
    decimals: jspb.Message.getFieldWithDefault(msg, 5, 0),
    groupTokenId: msg.getGroupTokenId_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpV1Nft1ChildGenesisMetadata}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpV1Nft1ChildGenesisMetadata;
  return proto.pb.SlpV1Nft1ChildGenesisMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpV1Nft1ChildGenesisMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpV1Nft1ChildGenesisMetadata}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setName(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTicker(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setDocumentUrl(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setDocumentHash(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setDecimals(value);
      break;
    case 6:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setGroupTokenId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpV1Nft1ChildGenesisMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpV1Nft1ChildGenesisMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getTicker_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
  f = message.getDocumentUrl_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getDocumentHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getDecimals();
  if (f !== 0) {
    writer.writeUint32(
      5,
      f
    );
  }
  f = message.getGroupTokenId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      6,
      f
    );
  }
};


/**
 * optional bytes name = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getName = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes name = 1;
 * This is a type-conversion wrapper around `getName()`
 * @return {string}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getName_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getName()));
};


/**
 * optional bytes name = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getName()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getName_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getName()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.setName = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional bytes ticker = 2;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getTicker = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes ticker = 2;
 * This is a type-conversion wrapper around `getTicker()`
 * @return {string}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getTicker_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTicker()));
};


/**
 * optional bytes ticker = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTicker()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getTicker_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTicker()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.setTicker = function(value) {
  jspb.Message.setProto3BytesField(this, 2, value);
};


/**
 * optional bytes document_url = 3;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getDocumentUrl = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes document_url = 3;
 * This is a type-conversion wrapper around `getDocumentUrl()`
 * @return {string}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getDocumentUrl_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getDocumentUrl()));
};


/**
 * optional bytes document_url = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getDocumentUrl()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getDocumentUrl_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getDocumentUrl()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.setDocumentUrl = function(value) {
  jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional bytes document_hash = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getDocumentHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes document_hash = 4;
 * This is a type-conversion wrapper around `getDocumentHash()`
 * @return {string}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getDocumentHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getDocumentHash()));
};


/**
 * optional bytes document_hash = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getDocumentHash()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getDocumentHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getDocumentHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.setDocumentHash = function(value) {
  jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * optional uint32 decimals = 5;
 * @return {number}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getDecimals = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.setDecimals = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bytes group_token_id = 6;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getGroupTokenId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * optional bytes group_token_id = 6;
 * This is a type-conversion wrapper around `getGroupTokenId()`
 * @return {string}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getGroupTokenId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getGroupTokenId()));
};


/**
 * optional bytes group_token_id = 6;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getGroupTokenId()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.getGroupTokenId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getGroupTokenId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1Nft1ChildGenesisMetadata.prototype.setGroupTokenId = function(value) {
  jspb.Message.setProto3BytesField(this, 6, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpV1Nft1ChildSendMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SlpV1Nft1ChildSendMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpV1Nft1ChildSendMetadata.displayName = 'proto.pb.SlpV1Nft1ChildSendMetadata';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpV1Nft1ChildSendMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpV1Nft1ChildSendMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpV1Nft1ChildSendMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1Nft1ChildSendMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    groupTokenId: msg.getGroupTokenId_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpV1Nft1ChildSendMetadata}
 */
proto.pb.SlpV1Nft1ChildSendMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpV1Nft1ChildSendMetadata;
  return proto.pb.SlpV1Nft1ChildSendMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpV1Nft1ChildSendMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpV1Nft1ChildSendMetadata}
 */
proto.pb.SlpV1Nft1ChildSendMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setGroupTokenId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpV1Nft1ChildSendMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpV1Nft1ChildSendMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpV1Nft1ChildSendMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpV1Nft1ChildSendMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGroupTokenId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes group_token_id = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpV1Nft1ChildSendMetadata.prototype.getGroupTokenId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes group_token_id = 1;
 * This is a type-conversion wrapper around `getGroupTokenId()`
 * @return {string}
 */
proto.pb.SlpV1Nft1ChildSendMetadata.prototype.getGroupTokenId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getGroupTokenId()));
};


/**
 * optional bytes group_token_id = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getGroupTokenId()`
 * @return {!Uint8Array}
 */
proto.pb.SlpV1Nft1ChildSendMetadata.prototype.getGroupTokenId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getGroupTokenId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpV1Nft1ChildSendMetadata.prototype.setGroupTokenId = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpTokenMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.SlpTokenMetadata.oneofGroups_);
};
goog.inherits(proto.pb.SlpTokenMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpTokenMetadata.displayName = 'proto.pb.SlpTokenMetadata';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.SlpTokenMetadata.oneofGroups_ = [[3,4,5]];

/**
 * @enum {number}
 */
proto.pb.SlpTokenMetadata.TypeMetadataCase = {
  TYPE_METADATA_NOT_SET: 0,
  V1_FUNGIBLE: 3,
  V1_NFT1_GROUP: 4,
  V1_NFT1_CHILD: 5
};

/**
 * @return {proto.pb.SlpTokenMetadata.TypeMetadataCase}
 */
proto.pb.SlpTokenMetadata.prototype.getTypeMetadataCase = function() {
  return /** @type {proto.pb.SlpTokenMetadata.TypeMetadataCase} */(jspb.Message.computeOneofCase(this, proto.pb.SlpTokenMetadata.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpTokenMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpTokenMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpTokenMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTokenMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    tokenId: msg.getTokenId_asB64(),
    tokenType: jspb.Message.getFieldWithDefault(msg, 2, 0),
    v1Fungible: (f = msg.getV1Fungible()) && proto.pb.SlpTokenMetadata.V1Fungible.toObject(includeInstance, f),
    v1Nft1Group: (f = msg.getV1Nft1Group()) && proto.pb.SlpTokenMetadata.V1NFT1Group.toObject(includeInstance, f),
    v1Nft1Child: (f = msg.getV1Nft1Child()) && proto.pb.SlpTokenMetadata.V1NFT1Child.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpTokenMetadata}
 */
proto.pb.SlpTokenMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpTokenMetadata;
  return proto.pb.SlpTokenMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpTokenMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpTokenMetadata}
 */
proto.pb.SlpTokenMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenId(value);
      break;
    case 2:
      var value = /** @type {!proto.pb.SlpTokenType} */ (reader.readEnum());
      msg.setTokenType(value);
      break;
    case 3:
      var value = new proto.pb.SlpTokenMetadata.V1Fungible;
      reader.readMessage(value,proto.pb.SlpTokenMetadata.V1Fungible.deserializeBinaryFromReader);
      msg.setV1Fungible(value);
      break;
    case 4:
      var value = new proto.pb.SlpTokenMetadata.V1NFT1Group;
      reader.readMessage(value,proto.pb.SlpTokenMetadata.V1NFT1Group.deserializeBinaryFromReader);
      msg.setV1Nft1Group(value);
      break;
    case 5:
      var value = new proto.pb.SlpTokenMetadata.V1NFT1Child;
      reader.readMessage(value,proto.pb.SlpTokenMetadata.V1NFT1Child.deserializeBinaryFromReader);
      msg.setV1Nft1Child(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpTokenMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpTokenMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTokenMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTokenId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getTokenType();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getV1Fungible();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.pb.SlpTokenMetadata.V1Fungible.serializeBinaryToWriter
    );
  }
  f = message.getV1Nft1Group();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.pb.SlpTokenMetadata.V1NFT1Group.serializeBinaryToWriter
    );
  }
  f = message.getV1Nft1Child();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.pb.SlpTokenMetadata.V1NFT1Child.serializeBinaryToWriter
    );
  }
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpTokenMetadata.V1Fungible = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SlpTokenMetadata.V1Fungible, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpTokenMetadata.V1Fungible.displayName = 'proto.pb.SlpTokenMetadata.V1Fungible';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpTokenMetadata.V1Fungible.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpTokenMetadata.V1Fungible} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTokenMetadata.V1Fungible.toObject = function(includeInstance, msg) {
  var f, obj = {
    tokenTicker: jspb.Message.getFieldWithDefault(msg, 1, ""),
    tokenName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    tokenDocumentUrl: jspb.Message.getFieldWithDefault(msg, 3, ""),
    tokenDocumentHash: msg.getTokenDocumentHash_asB64(),
    decimals: jspb.Message.getFieldWithDefault(msg, 5, 0),
    mintBatonHash: msg.getMintBatonHash_asB64(),
    mintBatonVout: jspb.Message.getFieldWithDefault(msg, 7, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpTokenMetadata.V1Fungible}
 */
proto.pb.SlpTokenMetadata.V1Fungible.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpTokenMetadata.V1Fungible;
  return proto.pb.SlpTokenMetadata.V1Fungible.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpTokenMetadata.V1Fungible} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpTokenMetadata.V1Fungible}
 */
proto.pb.SlpTokenMetadata.V1Fungible.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenTicker(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenDocumentUrl(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenDocumentHash(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setDecimals(value);
      break;
    case 6:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setMintBatonHash(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setMintBatonVout(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpTokenMetadata.V1Fungible.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpTokenMetadata.V1Fungible} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTokenMetadata.V1Fungible.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTokenTicker();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTokenName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTokenDocumentUrl();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getTokenDocumentHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getDecimals();
  if (f !== 0) {
    writer.writeUint32(
      5,
      f
    );
  }
  f = message.getMintBatonHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      6,
      f
    );
  }
  f = message.getMintBatonVout();
  if (f !== 0) {
    writer.writeUint32(
      7,
      f
    );
  }
};


/**
 * optional string token_ticker = 1;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getTokenTicker = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.setTokenTicker = function(value) {
  jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string token_name = 2;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getTokenName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.setTokenName = function(value) {
  jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string token_document_url = 3;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getTokenDocumentUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.setTokenDocumentUrl = function(value) {
  jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bytes token_document_hash = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getTokenDocumentHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes token_document_hash = 4;
 * This is a type-conversion wrapper around `getTokenDocumentHash()`
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getTokenDocumentHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenDocumentHash()));
};


/**
 * optional bytes token_document_hash = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenDocumentHash()`
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getTokenDocumentHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenDocumentHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.setTokenDocumentHash = function(value) {
  jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * optional uint32 decimals = 5;
 * @return {number}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getDecimals = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.setDecimals = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bytes mint_baton_hash = 6;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getMintBatonHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * optional bytes mint_baton_hash = 6;
 * This is a type-conversion wrapper around `getMintBatonHash()`
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getMintBatonHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getMintBatonHash()));
};


/**
 * optional bytes mint_baton_hash = 6;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getMintBatonHash()`
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getMintBatonHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getMintBatonHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.setMintBatonHash = function(value) {
  jspb.Message.setProto3BytesField(this, 6, value);
};


/**
 * optional uint32 mint_baton_vout = 7;
 * @return {number}
 */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.getMintBatonVout = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/** @param {number} value */
proto.pb.SlpTokenMetadata.V1Fungible.prototype.setMintBatonVout = function(value) {
  jspb.Message.setProto3IntField(this, 7, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpTokenMetadata.V1NFT1Group = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SlpTokenMetadata.V1NFT1Group, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpTokenMetadata.V1NFT1Group.displayName = 'proto.pb.SlpTokenMetadata.V1NFT1Group';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpTokenMetadata.V1NFT1Group.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpTokenMetadata.V1NFT1Group} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.toObject = function(includeInstance, msg) {
  var f, obj = {
    tokenTicker: jspb.Message.getFieldWithDefault(msg, 1, ""),
    tokenName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    tokenDocumentUrl: jspb.Message.getFieldWithDefault(msg, 3, ""),
    tokenDocumentHash: msg.getTokenDocumentHash_asB64(),
    decimals: jspb.Message.getFieldWithDefault(msg, 5, 0),
    mintBatonHash: msg.getMintBatonHash_asB64(),
    mintBatonVout: jspb.Message.getFieldWithDefault(msg, 7, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpTokenMetadata.V1NFT1Group}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpTokenMetadata.V1NFT1Group;
  return proto.pb.SlpTokenMetadata.V1NFT1Group.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpTokenMetadata.V1NFT1Group} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpTokenMetadata.V1NFT1Group}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenTicker(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenDocumentUrl(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenDocumentHash(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setDecimals(value);
      break;
    case 6:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setMintBatonHash(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setMintBatonVout(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpTokenMetadata.V1NFT1Group.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpTokenMetadata.V1NFT1Group} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTokenTicker();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTokenName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTokenDocumentUrl();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getTokenDocumentHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getDecimals();
  if (f !== 0) {
    writer.writeUint32(
      5,
      f
    );
  }
  f = message.getMintBatonHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      6,
      f
    );
  }
  f = message.getMintBatonVout();
  if (f !== 0) {
    writer.writeUint32(
      7,
      f
    );
  }
};


/**
 * optional string token_ticker = 1;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getTokenTicker = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.setTokenTicker = function(value) {
  jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string token_name = 2;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getTokenName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.setTokenName = function(value) {
  jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string token_document_url = 3;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getTokenDocumentUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.setTokenDocumentUrl = function(value) {
  jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bytes token_document_hash = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getTokenDocumentHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes token_document_hash = 4;
 * This is a type-conversion wrapper around `getTokenDocumentHash()`
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getTokenDocumentHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenDocumentHash()));
};


/**
 * optional bytes token_document_hash = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenDocumentHash()`
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getTokenDocumentHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenDocumentHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.setTokenDocumentHash = function(value) {
  jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * optional uint32 decimals = 5;
 * @return {number}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getDecimals = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.setDecimals = function(value) {
  jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bytes mint_baton_hash = 6;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getMintBatonHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * optional bytes mint_baton_hash = 6;
 * This is a type-conversion wrapper around `getMintBatonHash()`
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getMintBatonHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getMintBatonHash()));
};


/**
 * optional bytes mint_baton_hash = 6;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getMintBatonHash()`
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getMintBatonHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getMintBatonHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.setMintBatonHash = function(value) {
  jspb.Message.setProto3BytesField(this, 6, value);
};


/**
 * optional uint32 mint_baton_vout = 7;
 * @return {number}
 */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.getMintBatonVout = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/** @param {number} value */
proto.pb.SlpTokenMetadata.V1NFT1Group.prototype.setMintBatonVout = function(value) {
  jspb.Message.setProto3IntField(this, 7, value);
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpTokenMetadata.V1NFT1Child = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.pb.SlpTokenMetadata.V1NFT1Child, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpTokenMetadata.V1NFT1Child.displayName = 'proto.pb.SlpTokenMetadata.V1NFT1Child';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpTokenMetadata.V1NFT1Child.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpTokenMetadata.V1NFT1Child} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.toObject = function(includeInstance, msg) {
  var f, obj = {
    tokenTicker: jspb.Message.getFieldWithDefault(msg, 1, ""),
    tokenName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    tokenDocumentUrl: jspb.Message.getFieldWithDefault(msg, 3, ""),
    tokenDocumentHash: msg.getTokenDocumentHash_asB64(),
    groupId: msg.getGroupId_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpTokenMetadata.V1NFT1Child}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpTokenMetadata.V1NFT1Child;
  return proto.pb.SlpTokenMetadata.V1NFT1Child.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpTokenMetadata.V1NFT1Child} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpTokenMetadata.V1NFT1Child}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenTicker(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setTokenDocumentUrl(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenDocumentHash(value);
      break;
    case 5:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setGroupId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpTokenMetadata.V1NFT1Child.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpTokenMetadata.V1NFT1Child} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTokenTicker();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTokenName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTokenDocumentUrl();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getTokenDocumentHash_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getGroupId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      5,
      f
    );
  }
};


/**
 * optional string token_ticker = 1;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getTokenTicker = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.setTokenTicker = function(value) {
  jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string token_name = 2;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getTokenName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.setTokenName = function(value) {
  jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string token_document_url = 3;
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getTokenDocumentUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/** @param {string} value */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.setTokenDocumentUrl = function(value) {
  jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bytes token_document_hash = 4;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getTokenDocumentHash = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes token_document_hash = 4;
 * This is a type-conversion wrapper around `getTokenDocumentHash()`
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getTokenDocumentHash_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenDocumentHash()));
};


/**
 * optional bytes token_document_hash = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenDocumentHash()`
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getTokenDocumentHash_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenDocumentHash()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.setTokenDocumentHash = function(value) {
  jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * optional bytes group_id = 5;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getGroupId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * optional bytes group_id = 5;
 * This is a type-conversion wrapper around `getGroupId()`
 * @return {string}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getGroupId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getGroupId()));
};


/**
 * optional bytes group_id = 5;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getGroupId()`
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.getGroupId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getGroupId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpTokenMetadata.V1NFT1Child.prototype.setGroupId = function(value) {
  jspb.Message.setProto3BytesField(this, 5, value);
};


/**
 * optional bytes token_id = 1;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpTokenMetadata.prototype.getTokenId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes token_id = 1;
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {string}
 */
proto.pb.SlpTokenMetadata.prototype.getTokenId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenId()));
};


/**
 * optional bytes token_id = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {!Uint8Array}
 */
proto.pb.SlpTokenMetadata.prototype.getTokenId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpTokenMetadata.prototype.setTokenId = function(value) {
  jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional SlpTokenType token_type = 2;
 * @return {!proto.pb.SlpTokenType}
 */
proto.pb.SlpTokenMetadata.prototype.getTokenType = function() {
  return /** @type {!proto.pb.SlpTokenType} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {!proto.pb.SlpTokenType} value */
proto.pb.SlpTokenMetadata.prototype.setTokenType = function(value) {
  jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional V1Fungible v1_fungible = 3;
 * @return {?proto.pb.SlpTokenMetadata.V1Fungible}
 */
proto.pb.SlpTokenMetadata.prototype.getV1Fungible = function() {
  return /** @type{?proto.pb.SlpTokenMetadata.V1Fungible} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpTokenMetadata.V1Fungible, 3));
};


/** @param {?proto.pb.SlpTokenMetadata.V1Fungible|undefined} value */
proto.pb.SlpTokenMetadata.prototype.setV1Fungible = function(value) {
  jspb.Message.setOneofWrapperField(this, 3, proto.pb.SlpTokenMetadata.oneofGroups_[0], value);
};


proto.pb.SlpTokenMetadata.prototype.clearV1Fungible = function() {
  this.setV1Fungible(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpTokenMetadata.prototype.hasV1Fungible = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional V1NFT1Group v1_nft1_group = 4;
 * @return {?proto.pb.SlpTokenMetadata.V1NFT1Group}
 */
proto.pb.SlpTokenMetadata.prototype.getV1Nft1Group = function() {
  return /** @type{?proto.pb.SlpTokenMetadata.V1NFT1Group} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpTokenMetadata.V1NFT1Group, 4));
};


/** @param {?proto.pb.SlpTokenMetadata.V1NFT1Group|undefined} value */
proto.pb.SlpTokenMetadata.prototype.setV1Nft1Group = function(value) {
  jspb.Message.setOneofWrapperField(this, 4, proto.pb.SlpTokenMetadata.oneofGroups_[0], value);
};


proto.pb.SlpTokenMetadata.prototype.clearV1Nft1Group = function() {
  this.setV1Nft1Group(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpTokenMetadata.prototype.hasV1Nft1Group = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional V1NFT1Child v1_nft1_child = 5;
 * @return {?proto.pb.SlpTokenMetadata.V1NFT1Child}
 */
proto.pb.SlpTokenMetadata.prototype.getV1Nft1Child = function() {
  return /** @type{?proto.pb.SlpTokenMetadata.V1NFT1Child} */ (
    jspb.Message.getWrapperField(this, proto.pb.SlpTokenMetadata.V1NFT1Child, 5));
};


/** @param {?proto.pb.SlpTokenMetadata.V1NFT1Child|undefined} value */
proto.pb.SlpTokenMetadata.prototype.setV1Nft1Child = function(value) {
  jspb.Message.setOneofWrapperField(this, 5, proto.pb.SlpTokenMetadata.oneofGroups_[0], value);
};


proto.pb.SlpTokenMetadata.prototype.clearV1Nft1Child = function() {
  this.setV1Nft1Child(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpTokenMetadata.prototype.hasV1Nft1Child = function() {
  return jspb.Message.getField(this, 5) != null;
};



/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.pb.SlpRequiredBurn = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.pb.SlpRequiredBurn.oneofGroups_);
};
goog.inherits(proto.pb.SlpRequiredBurn, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.pb.SlpRequiredBurn.displayName = 'proto.pb.SlpRequiredBurn';
}
/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.pb.SlpRequiredBurn.oneofGroups_ = [[4,5]];

/**
 * @enum {number}
 */
proto.pb.SlpRequiredBurn.BurnIntentionCase = {
  BURN_INTENTION_NOT_SET: 0,
  AMOUNT: 4,
  MINT_BATON_VOUT: 5
};

/**
 * @return {proto.pb.SlpRequiredBurn.BurnIntentionCase}
 */
proto.pb.SlpRequiredBurn.prototype.getBurnIntentionCase = function() {
  return /** @type {proto.pb.SlpRequiredBurn.BurnIntentionCase} */(jspb.Message.computeOneofCase(this, proto.pb.SlpRequiredBurn.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.pb.SlpRequiredBurn.prototype.toObject = function(opt_includeInstance) {
  return proto.pb.SlpRequiredBurn.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.pb.SlpRequiredBurn} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpRequiredBurn.toObject = function(includeInstance, msg) {
  var f, obj = {
    outpoint: (f = msg.getOutpoint()) && proto.pb.Transaction.Input.Outpoint.toObject(includeInstance, f),
    tokenId: msg.getTokenId_asB64(),
    tokenType: jspb.Message.getFieldWithDefault(msg, 3, 0),
    amount: jspb.Message.getFieldWithDefault(msg, 4, "0"),
    mintBatonVout: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.pb.SlpRequiredBurn}
 */
proto.pb.SlpRequiredBurn.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.pb.SlpRequiredBurn;
  return proto.pb.SlpRequiredBurn.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.pb.SlpRequiredBurn} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.pb.SlpRequiredBurn}
 */
proto.pb.SlpRequiredBurn.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.pb.Transaction.Input.Outpoint;
      reader.readMessage(value,proto.pb.Transaction.Input.Outpoint.deserializeBinaryFromReader);
      msg.setOutpoint(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setTokenId(value);
      break;
    case 3:
      var value = /** @type {!proto.pb.SlpTokenType} */ (reader.readEnum());
      msg.setTokenType(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readUint64String());
      msg.setAmount(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setMintBatonVout(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.pb.SlpRequiredBurn.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.pb.SlpRequiredBurn.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.pb.SlpRequiredBurn} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.pb.SlpRequiredBurn.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOutpoint();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.pb.Transaction.Input.Outpoint.serializeBinaryToWriter
    );
  }
  f = message.getTokenId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
  f = message.getTokenType();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = /** @type {string} */ (jspb.Message.getField(message, 4));
  if (f != null) {
    writer.writeUint64String(
      4,
      f
    );
  }
  f = /** @type {number} */ (jspb.Message.getField(message, 5));
  if (f != null) {
    writer.writeUint32(
      5,
      f
    );
  }
};


/**
 * optional Transaction.Input.Outpoint outpoint = 1;
 * @return {?proto.pb.Transaction.Input.Outpoint}
 */
proto.pb.SlpRequiredBurn.prototype.getOutpoint = function() {
  return /** @type{?proto.pb.Transaction.Input.Outpoint} */ (
    jspb.Message.getWrapperField(this, proto.pb.Transaction.Input.Outpoint, 1));
};


/** @param {?proto.pb.Transaction.Input.Outpoint|undefined} value */
proto.pb.SlpRequiredBurn.prototype.setOutpoint = function(value) {
  jspb.Message.setWrapperField(this, 1, value);
};


proto.pb.SlpRequiredBurn.prototype.clearOutpoint = function() {
  this.setOutpoint(undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpRequiredBurn.prototype.hasOutpoint = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bytes token_id = 2;
 * @return {!(string|Uint8Array)}
 */
proto.pb.SlpRequiredBurn.prototype.getTokenId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes token_id = 2;
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {string}
 */
proto.pb.SlpRequiredBurn.prototype.getTokenId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getTokenId()));
};


/**
 * optional bytes token_id = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getTokenId()`
 * @return {!Uint8Array}
 */
proto.pb.SlpRequiredBurn.prototype.getTokenId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getTokenId()));
};


/** @param {!(string|Uint8Array)} value */
proto.pb.SlpRequiredBurn.prototype.setTokenId = function(value) {
  jspb.Message.setProto3BytesField(this, 2, value);
};


/**
 * optional SlpTokenType token_type = 3;
 * @return {!proto.pb.SlpTokenType}
 */
proto.pb.SlpRequiredBurn.prototype.getTokenType = function() {
  return /** @type {!proto.pb.SlpTokenType} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/** @param {!proto.pb.SlpTokenType} value */
proto.pb.SlpRequiredBurn.prototype.setTokenType = function(value) {
  jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional uint64 amount = 4;
 * @return {string}
 */
proto.pb.SlpRequiredBurn.prototype.getAmount = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, "0"));
};


/** @param {string} value */
proto.pb.SlpRequiredBurn.prototype.setAmount = function(value) {
  jspb.Message.setOneofField(this, 4, proto.pb.SlpRequiredBurn.oneofGroups_[0], value);
};


proto.pb.SlpRequiredBurn.prototype.clearAmount = function() {
  jspb.Message.setOneofField(this, 4, proto.pb.SlpRequiredBurn.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpRequiredBurn.prototype.hasAmount = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional uint32 mint_baton_vout = 5;
 * @return {number}
 */
proto.pb.SlpRequiredBurn.prototype.getMintBatonVout = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/** @param {number} value */
proto.pb.SlpRequiredBurn.prototype.setMintBatonVout = function(value) {
  jspb.Message.setOneofField(this, 5, proto.pb.SlpRequiredBurn.oneofGroups_[0], value);
};


proto.pb.SlpRequiredBurn.prototype.clearMintBatonVout = function() {
  jspb.Message.setOneofField(this, 5, proto.pb.SlpRequiredBurn.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {!boolean}
 */
proto.pb.SlpRequiredBurn.prototype.hasMintBatonVout = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * @enum {number}
 */
proto.pb.SlpTokenType = {
  VERSION_NOT_SET: 0,
  V1_FUNGIBLE: 1,
  V1_NFT1_CHILD: 65,
  V1_NFT1_GROUP: 129
};

/**
 * @enum {number}
 */
proto.pb.SlpAction = {
  NON_SLP: 0,
  NON_SLP_BURN: 1,
  SLP_PARSE_ERROR: 2,
  SLP_UNSUPPORTED_VERSION: 3,
  SLP_V1_GENESIS: 4,
  SLP_V1_MINT: 5,
  SLP_V1_SEND: 6,
  SLP_V1_NFT1_GROUP_GENESIS: 7,
  SLP_V1_NFT1_GROUP_MINT: 8,
  SLP_V1_NFT1_GROUP_SEND: 9,
  SLP_V1_NFT1_UNIQUE_CHILD_GENESIS: 10,
  SLP_V1_NFT1_UNIQUE_CHILD_SEND: 11
};

goog.object.extend(exports, proto.pb);
