# Intuition: Consensus on Luck

In our view, Bitcoin's pioneer practice on maintaining ``a single lane
quiet message dissemination traffic for a long time and for most of
the times'', although having a very low efficiency, should be very
useful for the BFT area of study to rethink of on what they really
mean by message exchange, discussion, and even to be consensus
agreed upon. The LotMint blockchain, which we believe to be a BFT
consensus protocol, shall do so on reformulating its BFT consensus. To
increase efficiency, we shall parallel message dissemination, and even
encourage a sizeable but non-flooding volume of message dissemination
traffic. What our BFT consensus shall agree upon is {\em not} on any
describable syntactic or semantic sense of a message, but on a much
vaguer idea that a node, upon hearing a message, can locally judge
deeming whether or not the message is {\em in-time lucky}. Let us
explain what we mean by an in-time lucky message.

A byproduct and also rather easy consensus for Bitcoin miners to reach
is block timestamps which are recorded in Bitcoin blocks. In
Section~\ref{BitcoinTimestamp} we shall see that Bitcoin has specified
block timestamp with very large room for error toleration. We must
pay attention to the following very important quality of Bitcoin block
timestamp: the consensus on agreeing the validity of block timestamps
is an in-writing form of reliability and with global visibility by all
nodes in the network, which have accepted the blocks containing the
timestamps. The time progressing gap between two consecutive
timestamps measures an average time length spent on mining plus
propagating a related block. This gap is therefore indeed a ``cycle''
for a global and logical clock. Thus, P2P network does can have a
global clock, although a quite imprecise one.

The LotMint blockchain shall make a good use of such an imprecise
global clock. However, we shall not let such an imprecise clock have
anything to involve in trying to make any syntactic or semantic sense
out of a message agreement consensus, or an ordered sequence out of
the time events of message dissemination (as having long been very
unsuccessfully attempted by many BFT consensus efforts). The use of
this imprecise global clock for LotMint is for figuring out a rather
imprecise scenario which we describe as follows. Let a node locally
judge and express that it hears a message ``in-time''. Upon such
imprecise in-time expressions reaching a BFT quorum, the in-time
message qualifies, again imprecisely, a time-tie set
membership. Fortunately, our imprecise timestamps defined imprecise
global clock is good enough to make and express imprecise in-time
judgments, which can effectively ``throttle'' the quantity of
applicants for the time-tie membership. Then, algorithmic ways to
order those time throttle passing non-populous time-tie members
abound.

We shall see that this ``consensus-on-luck'' formulation will have two
meanings for an in-time lucky message: (1) ``computation-lucky'' for
the message to have been created in time, and (2)
``communication-lucky'' for the message to have found a quick route to
reach recipients in time. Case (1) is for a miner to have PoW mined a
block luckily, and Case (2) is for a luckily mined block to have been
propagated through the P2P network luckily. In fact, the both
luckiness are possible thanks to the fact that these random time
variables belong to large deviation problems (LDPs) as we have
identified in Section~1. Unlike Bitcoin's setting a very highly cost
mining difficulty to avoid concurrent deviations, we shall choose to
feature concurrent deviations for high efficiency.

Interestingly and thankfully, ``hearing in-time messages by a BFT
quorum'', although not a very precise notion, is a meaningful
consensus to be agreed upon over a network containing faults. At the
birth of such a network, its architects had envisaged these faults and
conceived for it to keep being added on ampler and ampler, and more
and more redundant routes. To date and in future, this network has
been and will be evolving to ever growing likelihood that a message on
such a network may find luckier route(s) to reach its destination. It
is the ever growing redundant for this network that can make a message
to sometimes strike a time-tie luck, and even for the luck to be
distributively and independently agreed by a quorum of such redundant
nodes.

Perhaps, the BFT area of study might be added an optimistic and
opportunistic nickname, such as ``Lucky BFT'', for modifying consensus
protocols running on the imprecise and great network, i.e., the
internet, we are using?
