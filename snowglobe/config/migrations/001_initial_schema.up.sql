CREATE TABLE `blocks` (
  `hash` CHAR(64) NOT NULL PRIMARY KEY,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  --
  `previous_hash` CHAR(64) NOT NULL,
  `height` INT NOT NULL
);
CREATE TABLE `blocks_transactions` (
  `block_hash` CHAR(64) NOT NULL,
  `transaction_hash` CHAR(64) NOT NULL,
  PRIMARY KEY (`block_hash`, `transaction_hash`)
);
CREATE TABLE `transaction_inputs` (
  `transaction_hash` CHAR(64) NOT NULL,
  `outpoint_transaction_hash` CHAR(64) NOT NULL,
  `outpoint_index` INT NOT NULL,
  PRIMARY KEY (
    `transaction_hash`,
    `outpoint_transaction_hash`,
    `outpoint_index`
  )
);
CREATE TABLE `outpoints` (
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `transaction_hash` CHAR(64) NOT NULL,
  `index` INT NOT NULL,
  `amount` BIGINT NOT NULL,
  PRIMARY KEY (`transaction_hash`, `index`)
);
CREATE TABLE `peers` (
  `identity_key` CHAR(66) NOT NULL PRIMARY KEY,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:01',
  `sequence` INT NOT NULL,
  `signature` TEXT NOT NULL,
  `stake_message` TEXT NOT NULL
);
CREATE TABLE `vote_records` (
  `peer_identity_key` CHAR(66) NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  --
  `vertex_type` VARCHAR(255) NOT NULL,
  `vertex_hash` CHAR(64) NOT NULL,
  --
  `state` VARCHAR(255),
  `initial_state` VARCHAR(255),
  --
  `started_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:01',
  `finalized_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:01',
  PRIMARY KEY (`peer_identity_key`, `vertex_hash`)
);
CREATE TABLE `query` (
  `id` BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  `to_identity_key` CHAR(66) NOT NULL,
  `from_identity_key` CHAR(66) NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `response_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:01',
  `requested_count` INT NOT NULL DEFAULT 0,
  `response_count` INT NOT NULL DEFAULT 0
);