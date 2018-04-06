---
title: P2P Battle Royale
header-includes:
    - \author{Jerome Rasky (g8z9a), Madeleine Chercover (f4u9a), Raunak Kumar (i4x8), Vaastav Anand (s8a9)}
    - \usepackage{fancyhdr}
    - \pagestyle{fancy}
    - \fancyhead[LO, RE]{Jerome Rasky, Madeleine Chercover, Raunak Kumar, Vaastav Anand}
    - \fancyhead[LE, RO]{CPSC 416 Project 2 Final Report}
geometry: margin=1in
---

# Abstract

# Introduction

In recent years, a new genre of video games, often termed as 'battle royale', has become increasingly popular. Games of this genre are, in effect, last-man standing games – the last surviving player wins. Such games involve frequent altercations between players and consequently place heavy demands on how the game state is maintained. The world must appear consistent for all players as it is modified, interactions between players must be resolved fairly, and eliminated players should no longer be able to modify the game state.

For our final project, we built a distributed, 2D battle royale-style game. Players move, aim, and fire, within a shared, fixed-size map; the last player standing wins. In addition to being a popular source of entertainment, such games pose interesting challenges when designed and developed as a distributed system. Key among these challenges are (1) the amount of distributed state and (2) the requirement for low latency. Whereas a blockchain, for example, can reasonably take ten minutes to confirm transactions, players expect near-instantaneous interaction. We have sought to build our system with such requirements in mind.            

# Definitions

# Design

## Overview

## Server

The server is used only for those functions which are not latency-sensitive or which require consensus. Because there is not the same requirement for low latency that there is for interaction between player nodes (discussed below), communication with the server uses the Transmission Control Protocol (TPC). The server functions are as follows:

* __Peer Discovery__: The server returns a set of addresses of other player nodes in the network. The player then maintains a minimum number of peers, requesting more player addresses from the server as needed. On startup, player nodes must thus register their address with the server so that they are then discoverable by other nodes.

* __Player Reconnection__: When a player node disconnects, the server is notified of that failure. It then (1) stops returning the failed node's address in peer discovery and (2) begins monitoring that node in case it reconnects. In the event that the node reconnects, the server can then resume returning its address to other nodes. Node failure is discussed in more detail below.

* __Clock Synchronization__: Given that our game is a real-time distributed system, with player nodes broadcasting their moves and shots, we need a method by which to order updates and thereby resolve altercations between players. To do so, we use clock synchronization amongst all player nodes, and in particular the Berkeley Algorithm. For the purposes of this algorithm, the server is selected as master. *TODO*  

* __Key-Value Something Something__: *TODO*

### Server API

With those functions in mind, the API for communication with the server is as follows:

* __success, err ← Register(address, tcpAddress, clientId, displayName, logger)__: Registers the given `clientId`, and associated `displayName` and addresses with the server. The client may then further interact with the server in the calls below.
* __PeerNetSettings, err ← Connect(clientId, logger)__: Marks the player node associated with `clientId` as connected, allowing its address to be returned to other nodes in calls to `GetNodes()` (defined below). Returns the network settings for the game, including the minimum number of peers that a player node should maintain.
* __[]PeerInfo, err ← GetNodes(clientId, logger)__: Returns a set of addresses of player nodes that are currently marked as connected.
* __err ← NotifyFailure(clientId)__: Marks the player node associated with `clientId` as disconnected, so that it is no longer returned in calls to `GetNodes()`. The server then begins monitoring that node in the event that it reconnects.
* __value, err ← KVGet(key, clientId, logger)__: *TODO*
* __err ← KVPut(key, value, logger)__: *TODO*

## Player Node

### Player API

The API for communication with player nodes is as follows:

* __err ← Register(clientId, address, tcpAddress)__ : Add `clientId` to the receiving player's peer list, and begin sending heartbeats to `tcpAddress`.
* __err ← NotifyUpdate(clientId, update)__ : Update the game state with the given `update` and flood the update to the receiving player's peers.
* __err ← NotifyFailure(clientId, ttl)__ : Mark the player with `clientId` as dead in the game state, decrement `ttl`, and flood the failure to the receiving player's peers, unless `ttl` == 0.

* __time, err ← TimeRequest()__ : ...
* __err ← SetOffset(offset)__ : ...

* __err ← Heartbeat(clientId)__ : ...
* __err ← Ping()__ : No-op call used to test the connection between the caller and receiving player node.  
* __success, err ← Recover()__ : ...

* __value, err ← KVClientGet(key, logger)__ : ...
* __err ← KVClientPut(key, value, logger)__ : ...

### Node Joins

### Node Failures

## Stats Collection

# Implementation

## Azure

## Library Dependencies

# Limitations and Future Improvements

# Allocation of Work

Jerome Rasky worked on graphics, game mechanics, and player-to-player communication; Madeleine Chercover worked on player node failure detection, transitory disconnection handling, and the bot player; Raunak Kumar worked on the key-value store; Vaastav Anand worked on clock synchronization, client-server communication, and GoVector, Shiviz, and Dinv integration.
