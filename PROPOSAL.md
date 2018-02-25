---
title: CPSC 416 Project 2 Proposal
header-includes:
    - \author{Jerome Rasky, Madeleine Chercover, Raunak Kumar, Vaastav Anand}
    - \usepackage{fancyhdr}
    - \pagestyle{fancy}
    - \fancyhead[LO, RE]{Jerome Rasky, Madeline Chercover, Raunak Kumar, Vaastav Anand}
    - \fancyhead[LE, RO]{CPSC 416 Project 2 Proposal}
geometry: margin=1in
---
\pagebreak

# A Distributed Game

We're interested in building a distributed game for our term project. We feel that this area is interesting because it introduces real-time constraints into our distributed system. Namely, a game requires low latency. Whereas the blockchain can afford to take ten minutes to confirm transactions, players might be upset if it takes ten minutes to move at all in a game.

There is prior art in the area, and some professional games such as Destiny are based on distributed systems. Clock synchronization is likely to be a topic of interest in our system, which means that we can apply some of the topics we've learned in class. We'll have to do more research on how best to match peers, which might involve a central server like in project 1, or some kind of distributed way to bootstrap into the network.

In order to keep track of game state across the network, we might make use of conflict-free replicated data types, such as a vector clock. We'll have to work hard to ensure low latency, which will mean using networking tricks and whatever else is otherwise available to improve latency.

# Resources

<https://www.cs.ubc.ca/~gberseth/projects/ArmGame/ARM%20Game%20With%20Distributed%20States%20-%20Glen%20Berseth,%20Ravjot%20%20%20%20%20%20Singh.pdf>

<http://www.it.uom.gr/teaching/distrubutedSite/dsIdaLiu/lecture/lect11-12.frm.pdf>

<https://www.microsoft.com/en-us/research/uploads/prod/2016/12/Time-Clocks-and-the-Ordering-of-Events-in-a-Distributed-System.pdf>

<https://en.wikipedia.org/wiki/Berkeley_algorithm>

<http://pmg.csail.mit.edu/papers/osdi99.pdf>

<https://en.wikipedia.org/wiki/Conflict-free_replicated_data_type>
