Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Building Blocks

# Building Blocks

Building blocks are grouped in terms of research units and don't have a special
representation in code. Also, the _General Crypto_ library is more of a
collection of different protocols.

## Consensus

Whenever you have a distributed system the question of consensus arises: how do
you make sure that a majority of nodes see the same state? At DEDIS we're using
collective signatures as a base to build a PBFT-like consensus protocol.
Based on that we implemented a skipchain that can store arbitrary data in blocks
and use a verification function to make sure all nodes agree on every block.

- [Collective Signing](../blscosi/README.md)
is the current signing algorithm we are using (deprecates cosi and ftcosi).
- [ByzCoinX](../byzcoinx/README.md) is
the an improved implementation of the consensus protocol in the OmniLedger paper.

Deprecated:
- [Collective Signing](../cosi/README.md)
is the basic signing algorithm we've been using - replaced by:
- [Fault Tolerant Collective Signing](../ftcosi/README.md)
a fault tolerant version of the CoSi protocol with only a 3-level tree
- [Byzantine Fault Tolerant CoSi](../bftcosi/README.md)
is an implementation inspired by PBFT using two rounds of CoSi

## Key Sharding

Another useful tool for distributed system is key sharing, or key sharding.
Instead of having one key that can be compromised easily, we create an
aggregate key where a threshold number of nodes need to participate to
decrypt or sign. Our blocks can do a publicly verifiable distributed
key generation as well as use that sharded key to decrypt or reencrypt data
to a new key without ever having the data available.

- [Distributed Key Generation](../dkg/DKG.md)
uses the protocol presented by Rabin to create a distributed key
- [Distributed Decryption](../evoting/protocol/Decrypt.md)
takes an ElGamal encrypted ciphertext and decrypts it using nodes
- [Re-encryption](../calypso/protocol/Reencrypt.md)
re-encrypts an ElGamal encryption to a new key while never revealing the original
data

## Messaging

Finally some building blocks useful in most of the services.

- [Broadcast and Propagation](../messaging/README.md)
