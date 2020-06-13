# LotMint

Blockchain Returning to Decentralization with Decentralized Clock, forked from dedis/cothority project.

## Abstract

We present LotMint, a permissionless blockchain, with a purposely low
set bar for Proof-of-Work (PoW) difficulty. Our objective is for
personal computers, cloud virtual machines or containers, even mobile
devices, and hopefully future IoT devices, to become the main, widely
distributed, collectively much securer, fairer, more reliable and
economically sustainable mining workforce for blockchains. An
immediate question arises: how to prevent the permissionless network
from being flooded of block dissemination traffic by a massive number
of profit enthusiastic miners? We propose a novel notion of 
  Decentralized Clock/Time (DC/DT) as global and logical clock/time
which can be agreed upon as a consensus. Our construction of DC/DT
practically uses distributed private clocks of the participation
nodes. With DC/DT, a node upon creating or hearing a block can know
how luckily short or unluckily long time for the block to have been
mined and/or traveled through the network. They can ``time throttle''
a potentially large number of unluckily mined/travelled blocks. The
luckier blocks passing through the time throttle are treated as
time-tie forks with a volume being ``throttle diameter'' adjustably
controlled not to congest the network. With the number of time-tie
forks being manageable, it is then easy to break-tie elect a winner,
or even to orderly queue a plural number of winners for a
``multi-core'' utilization of resource. We provide succinct and
evident analyses of necessary properties for the LotMint blockchain
including: decentralization, energy saving, safety, liveness,
robustness, fairness, anti-denial-of-service, anti-sybil,
anti-censorship, scaled-up transaction processing throughput and
sped-up payment confirmation time.

## The LotMint Blockchain

The LotMint blockchain has made a number of changes to ByzCoin. Below we describe the LotMint blockchain with changes from ByzCoin being explained, and important security properties analyzed in enumeration.

1. The ﬁrst and the most important change is on the block mining mechanism. In LotMint, block mining shall use the decentralized clock enabled time throttle mechanism that we have described in Section 4. Each new epoch begins with a mining competition where the reference block RB that the miners follow is the latest won time block TB. The deﬁnition of TB shall be described in Item 9 of our enumeration description. A mining success output, which we shall follow ByzCoin’s (and Bitcoin-NG’s) naming of their “KeyBlocks”, to name it a “KeyBlock Transaction”, and denote it by KB TX. Notice that treating PoW mining blocks as transactions to disseminate in the network is novel in BFT+blockchain technologies. Thanks to our DT time-throttle control, the number of KB TXs in dissemination can be well controlled not to congest the network.


![LotMint Figure 1](https://lotmint.io/wp-content/uploads/2020/06/figure1.png)


![LotMint Figure 2](https://lotmint.io/wp-content/uploads/2020/06/epoch.png)

2. After elapsing of Φ + ∆ interval of time (see
    Section 3.4 for the meaning of Φ and ∆) from the
    GT written in the current reference block $\mathtt{RB}$, the
    network shall become in the absence of $\mathtt{KB\_TX}$. The
    current BFT leader shall propose a new ByzCoin CoSi tree to try to
    have all competing $\mathtt{KB\_TX}$s to be BFT quorum approved to
    enter the blockchain. Upon reaching the BFT quorum consensus, the
    root of this new CoSi tree becomes the new reference block
    $\mathtt{RB}$ for to be referenced by a new round of mining
    competition.

3. If the current BFT leader is detected to be censoring some transactions, including some KB TXs, the BFT leader is said to have made a “censorship-error” omission. Upon detecting a censorship-error omission, other BFT trustees shall follow the current CoSi tree to have the censorship omitted TXs and/or KB TXs to be BFT quorum approved to enter the blockchain. This “censorship-error” correction is shown by the “A SubLeader Led Authority” sub-CoSi-tree in Figure 2. A punitive de-incentive scheme shall apply to the BFT leader who is BFT quorum agreed to have factually made a censorship-error by the very existence of a sub-CoSi-tree. In case of a number of trustees competing in censorship error correction, the deterministic de-forking method of ByzCoin (Figure 5 of [15]) can break forks.

4. If the current leader is detected to be proposing a CoSi to contain any message with a safety error, other than a censorship-error case, this leader is said to have conducted a “safety-error” attack. Upon trustees detecting a safety-error attack, a BFT change of leader event shall take place. This safety-error correction procedure is the same as that ByzCoin’s change-of-leader event does.

5. The diameter of the time throttle, i.e., Φ deﬁned in Section 3.4, may be in consensus algorithm adjusted in real time. If the number of time-tie KB TXs forks is too small/big, then the consensus algorithm can adjust to enlarge/shrink the diameter Φ. Also, an epoch shall complete upon reaching a pre-determined desirable and suﬃcient number of time-tie KB TXs forks.

6. Time-tie $\mathtt{KB\_TX}$s are BFT approved by a quorum of
    trustees in the BFT consortium. In Section 2, we
    have intuitively attributed a BFT approval of $\mathtt{KB\_TX}$ to
    being ``lucky''. In Section 3.4, we further made the meaning
    of a lucky block specific in that it is both computation- and
    communication-lucky. It is now clear that a lucky block is
    possible because the random time variable of block arrival times 
    have large deviations. Our blockchain protocol choose to utilize
    these lucky events as a feature rather than to avoid them as
    problems. We further believe that, with our deliberately vague
    treatment on what to be BFT consensus agreed upon, the size of a
    ``lucky-quorum'' can also be a consensus algorithm adjustable
    variable, not necessary to be somewhat 2/3 fractions of the
    consortium size as in the case of pBFT. We believe that a simple
    majority can be a reasonable setting to achieve a quorum.

7. In ByzCoin, KeyBlock mining is in a separate chain, which, because a KeyBlock contains no transactions, seemingly to us is not very clear why other not-won miners shall have an eagerness to propagate a winner’s KeyBlock. To increase the blockchain liveness as Bitcoin made a seminal contribution to BFT protocols, we shall use an incentivization rewarding scheme, e.g., that of Bitcoin-NG [17], to divide mining incentive of the current epoch to reward the next epoch mining winner(s). The incentive reward should also include the coin-base minted coins in those not-won KB TXs to encourage miners to eagerly propagate these blocks. This is very important not only for the blockchain’s transaction quality of service viewed by the users, but also for liveness of the blockchain, as in the case of Bitcoin.

8. We have ﬂattened the much eased PoW mining opportunities evenly distributed to the entire duration of an epoch. Each root of a CoSi tree is viewed by the miners a new reference block RB to reference mining new blocks. All lucky KB TXs which references any RB anywhere in an epoch are time-tie forks, even though they may have forked diﬀerent RBs.

9. An epoch shall end upon reaching a pre-determined, and suﬃcient number of timetie KB TXs having entered BFT quorum approved CoSi trees. The ﬁnal CoSi tree in the epoch should be a CoSi on the KB TX which is deterministically de-forked from all quorum approved time-tie KB TXs in the current epoch, using the deterministic de-forking method of ByzCoin (see Figure 5 of [15]). This CoSi tree deﬁnes the time block TB for the next epoch, and it should contain a Bitcoin like block timestamp, to function deﬁning a new GC Cycle(TB) of the global and logic clock.

10. With the DT time-throttle method’s embracing mining time-tie forks, the deterministic de-forking method of ByzCoin can select a plural number of BFT leaders with an ordered winning sequence. They can be thought of as “multi-core” CPUs for strengthening the BFT consortium as additional trustees, e.g., to replace safety-error attacking leader(s). This way of strengthening the BFT trustees consortium can improve fairness, safety, liveness and robustness qualities of the blockchain, in addition to energy conservation.

11. With much lowered cost for being a miner, a LotMint miner may like to widely distribute itself over the network in replicas with a plural number of public-key based mining addresses being publicized by its won KB TX. Then, such a distributed miner, upon ﬁnding self being DoS attacked due to having disclosed an IP address, may change to a replica to continue working. The order of using replicas may follow the natural order of the public-key replicas.

It is our belief that the transaction processing scalability and the conﬁrmation time properties of the LotMint blockchain, are similar to those of ByzCoin.
