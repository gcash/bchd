#!/usr/bin/env python3

import grpc
import bchrpc_pb2 as pb
import bchrpc_pb2_grpc as bchrpc

def run():
    with grpc.secure_channel('bchd.greyh.at:8335', grpc.ssl_channel_credentials()) as channel:
        stub = bchrpc.bchrpcStub(channel)

        ## Fetch tx history for bch address
        address = "qrj279sdafw2r9zkcw7uncdhks2s3lpzmvwlle5243"

        req = pb.GetAddressTransactionsRequest()
        req.address = address

        resp = stub.GetAddressTransactions(req)

        print("Fetch tx history for address: " + address + "\n")
        # print(resp)
        for tx in resp.confirmed_transactions:
            incoming = 0
            outgoing = 0
            for txInput in tx.inputs:
                if (txInput.address == address):
                    outgoing += txInput.value

            for txOutput in tx.outputs:
                if (txOutput.address == address):
                    incoming += txOutput.value

            print("%s => INCOMING: %14d sats - OUTGOING: %14d sats" % (bytearray(tx.hash[::-1]).hex(), incoming, outgoing))

        print("\nUNCONFIRMED")
        for tx in resp.unconfirmed_transactions:
            incoming = 0
            outgoing = 0
            for txInput in tx.transaction.inputs:
                if (txInput.address == address):
                    outgoing += txInput.value

            for txOutput in tx.transaction.outputs:
                if (txOutput.address == address):
                    incoming += txOutput.value

            print("%s => INCOMING: %14d sats - OUTGOING: %14d sats" % (bytearray(tx.transaction.hash[::-1]).hex(), incoming, outgoing))


        ## Monitor address for incoming txs
        ## Ideally this should run in a separate thread since this is a blocking call
        txFilter = pb.TransactionFilter()
        txFilter.addresses.append(address)
        # txFilter.all_transactions = True

        req = pb.SubscribeTransactionsRequest()
        req.include_in_block = True
        req.include_mempool = True
        req.subscribe.CopyFrom(txFilter)

        print("\nMonitor txs for address: " + address + "\n")
        for notification in stub.SubscribeTransactions(req):
            tx = notification.unconfirmed_transaction.transaction

            incoming = 0
            outgoing = 0
            for txInput in tx.inputs:
                if (txInput.address == address):
                    outgoing += txInput.value

            for txOutput in tx.outputs:
                if (txOutput.address == address):
                    incoming += txOutput.value

            print("%s => INCOMING: %14d sats - OUTGOING: %14d sats" % (bytearray(tx.hash[::-1]).hex(), incoming, outgoing))


if __name__ == '__main__':
    run()
