package main

import "C"
import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

type Order struct {
	orderType   inputType
	orderId     uint32
	price       uint32
	count       uint32
	executionId uint32
	instrument  string
	timestamp   int64
}

type BuyOrderHeap []uint32

func (h BuyOrderHeap) Len() int { return len(h) }

func (h BuyOrderHeap) Less(i, j int) bool { return h[i] > h[j] }

func (h BuyOrderHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *BuyOrderHeap) Push(x interface{}) {
	*h = append(*h, x.(uint32))
}

func (h *BuyOrderHeap) Pop() interface{} {
	oldHeap := *h
	oldLength := len(oldHeap)
	x := oldHeap[oldLength-1]
	*h = oldHeap[0 : oldLength-1]
	return x
}

type SellOrderHeap []uint32

func (h SellOrderHeap) Len() int { return len(h) }

func (h SellOrderHeap) Less(i, j int) bool { return h[i] < h[j] }

func (h SellOrderHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *SellOrderHeap) Push(x interface{}) {
	*h = append(*h, x.(uint32))
}

func (h *SellOrderHeap) Pop() interface{} {
	oldHeap := *h
	oldLength := len(oldHeap)
	x := oldHeap[oldLength-1]
	*h = oldHeap[0 : oldLength-1]
	return x
}

type Wrapper struct {
	doneChannel chan struct{}
	orderInput  input
}

type Engine struct {
	wrapperChannel chan Wrapper
}

func newEngine() *Engine {
	e := &Engine{
		wrapperChannel: make(chan Wrapper),
	}
	go e.mainRoutine()
	return e
}

func (e *Engine) mainRoutine() {
	instrumentToChannelMap := make(map[string]chan Wrapper)
	orderIdToInputMap := make(map[uint32]input)
	for {
		wrapperInput := <-e.wrapperChannel
		in := wrapperInput.orderInput
		if in.orderType == inputCancel {
			in.instrument = orderIdToInputMap[in.orderId].instrument
		} else {
			orderIdToInputMap[in.orderId] = in
		}
		instrumentChannel, found := instrumentToChannelMap[in.instrument]
		if !found {
			instrumentChannel = make(chan Wrapper)
			instrumentToChannelMap[in.instrument] = instrumentChannel
			go instrumentRoutine(instrumentChannel)
		}
		instrumentChannel <- wrapperInput
	}
}

func instrumentRoutine(instrumentChannel chan Wrapper) {
	instrumentBuyMap := &BuyOrderHeap{}
	heap.Init(instrumentBuyMap)

	instrumentSellMap := &SellOrderHeap{}
	heap.Init(instrumentSellMap)

	buyPriceToQueue := make(map[uint32][]Order)

	sellPriceToQueue := make(map[uint32][]Order)

	orderIdToCountMap := make(map[uint32]uint32)

	for {
		wrapperInput := <-instrumentChannel
		in := wrapperInput.orderInput
		clientChannel := wrapperInput.doneChannel
		switch in.orderType {
		case inputBuy:
			activeBuy := Order{inputBuy, in.orderId, in.price, in.count, 1, in.instrument, GetCurrentTimestamp()}
			countToSatisfy := activeBuy.count

			if len(*instrumentSellMap) == 0 {
				outputOrderAdded(in, GetCurrentTimestamp())
				if _, found := buyPriceToQueue[activeBuy.price]; !found {
					heap.Push(instrumentBuyMap, activeBuy.price)
				}
				buyQueue, found := buyPriceToQueue[activeBuy.price]
				if !found {
					buyQueue = []Order{}
				}
				buyQueue = append(buyQueue, activeBuy)
				buyPriceToQueue[activeBuy.price] = buyQueue
				orderIdToCountMap[activeBuy.orderId] = activeBuy.count
				clientChannel <- struct{}{}
				continue
			}

			if (*instrumentSellMap)[0] > activeBuy.price {
				outputOrderAdded(in, GetCurrentTimestamp())
				if _, found := buyPriceToQueue[activeBuy.price]; !found {
					heap.Push(instrumentBuyMap, activeBuy.price)
				}
				buyQueue, found := buyPriceToQueue[activeBuy.price]
				if !found {
					buyQueue = []Order{}
				}
				buyQueue = append(buyQueue, activeBuy)
				buyPriceToQueue[activeBuy.price] = buyQueue
				orderIdToCountMap[activeBuy.orderId] = activeBuy.count
				clientChannel <- struct{}{}
				continue
			}

			for countToSatisfy > 0 {
				sellQueue := sellPriceToQueue[(*instrumentSellMap)[0]]
				sellOrder := &sellQueue[0]
				if orderIdToCountMap[sellOrder.orderId] == 0 {
					sellQueue = sellQueue[1:]
					sellPriceToQueue[sellOrder.price] = sellQueue
					if len(sellQueue) == 0 {
						delete(sellPriceToQueue, sellOrder.price)
						heap.Pop(instrumentSellMap)
						if len(*instrumentSellMap) == 0 || (*instrumentSellMap)[0] > activeBuy.price {
							break
						}
					}
					continue
				}
				amountSold := min(countToSatisfy, orderIdToCountMap[sellOrder.orderId])
				orderIdToCountMap[sellOrder.orderId] -= amountSold
				outputOrderExecuted(
					sellOrder.orderId,
					activeBuy.orderId,
					sellOrder.executionId,
					sellOrder.price,
					amountSold,
					GetCurrentTimestamp(),
				)
				sellOrder.executionId++
				countToSatisfy -= amountSold
				if orderIdToCountMap[sellOrder.orderId] == 0 {
					sellQueue = sellQueue[1:]
					sellPriceToQueue[sellOrder.price] = sellQueue
				}
				if len(sellQueue) == 0 {
					delete(sellPriceToQueue, sellOrder.price)
					heap.Pop(instrumentSellMap)
					if len(*instrumentSellMap) == 0 || (*instrumentSellMap)[0] > activeBuy.price {
						break
					}
				}
			}

			if countToSatisfy != 0 {
				activeBuy.count = countToSatisfy
				in.count = countToSatisfy
				outputOrderAdded(in, GetCurrentTimestamp())
				if _, found := buyPriceToQueue[activeBuy.price]; !found {
					heap.Push(instrumentBuyMap, activeBuy.price)
				}
				buyQueue, found := buyPriceToQueue[activeBuy.price]
				if !found {
					buyQueue = []Order{}
				}
				buyQueue = append(buyQueue, activeBuy)
				buyPriceToQueue[activeBuy.price] = buyQueue
				orderIdToCountMap[activeBuy.orderId] = activeBuy.count
			}
			clientChannel <- struct{}{}

		case inputSell:
			activeSell := Order{inputSell, in.orderId, in.price, in.count, 1, in.instrument, GetCurrentTimestamp()}
			countToSatisfy := activeSell.count
			if len(*instrumentBuyMap) == 0 {
				outputOrderAdded(in, GetCurrentTimestamp())
				if _, found := sellPriceToQueue[activeSell.price]; !found {
					heap.Push(instrumentSellMap, activeSell.price)
				}
				sellQueue, found := sellPriceToQueue[activeSell.price]
				if !found {
					sellQueue = []Order{}
				}
				sellQueue = append(sellQueue, activeSell)
				sellPriceToQueue[activeSell.price] = sellQueue
				orderIdToCountMap[activeSell.orderId] = activeSell.count
				clientChannel <- struct{}{}
				continue
			}
			if (*instrumentBuyMap)[0] < activeSell.price {
				outputOrderAdded(in, GetCurrentTimestamp())
				if _, found := sellPriceToQueue[activeSell.price]; !found {
					heap.Push(instrumentSellMap, activeSell.price)
				}
				sellQueue, found := sellPriceToQueue[activeSell.price]
				if !found {
					sellQueue = []Order{}
				}
				sellQueue = append(sellQueue, activeSell)
				sellPriceToQueue[activeSell.price] = sellQueue
				orderIdToCountMap[activeSell.orderId] = activeSell.count
				clientChannel <- struct{}{}
				continue
			}
			for countToSatisfy > 0 {
				buyQueue := buyPriceToQueue[(*instrumentBuyMap)[0]]
				buyOrder := &buyQueue[0]
				if orderIdToCountMap[buyOrder.orderId] == 0 {
					buyQueue = buyQueue[1:]
					buyPriceToQueue[buyOrder.price] = buyQueue
					if len(buyQueue) == 0 {
						delete(buyPriceToQueue, buyOrder.price)
						heap.Pop(instrumentBuyMap)
						if len(*instrumentBuyMap) == 0 || (*instrumentBuyMap)[0] < activeSell.price {
							break
						}
					}
					continue
				}
				amountSold := min(countToSatisfy, orderIdToCountMap[buyOrder.orderId])
				orderIdToCountMap[buyOrder.orderId] -= amountSold
				outputOrderExecuted(
					buyOrder.orderId,
					activeSell.orderId,
					buyOrder.executionId,
					buyOrder.price,
					amountSold,
					GetCurrentTimestamp(),
				)
				buyOrder.executionId++
				countToSatisfy -= amountSold
				if orderIdToCountMap[buyOrder.orderId] == 0 {
					buyQueue = buyQueue[1:]
					buyPriceToQueue[buyOrder.price] = buyQueue
				}
				if len(buyQueue) == 0 {
					delete(buyPriceToQueue, buyOrder.price)
					heap.Pop(instrumentBuyMap)
					if len(*instrumentBuyMap) == 0 || (*instrumentBuyMap)[0] < activeSell.price {
						break
					}
				}
			}

			if countToSatisfy != 0 {
				activeSell.count = countToSatisfy
				in.count = countToSatisfy
				outputOrderAdded(in, GetCurrentTimestamp())
				if _, found := sellPriceToQueue[activeSell.price]; !found {
					heap.Push(instrumentSellMap, activeSell.price)
				}
				sellQueue, found := sellPriceToQueue[activeSell.price]
				if !found {
					sellQueue = []Order{}
				}
				sellQueue = append(sellQueue, activeSell)
				sellPriceToQueue[activeSell.price] = sellQueue
				orderIdToCountMap[activeSell.orderId] = activeSell.count
			}
			clientChannel <- struct{}{}

		case inputCancel:
			count, found := orderIdToCountMap[in.orderId]
			if !found {
				outputOrderDeleted(in, false, GetCurrentTimestamp())
				clientChannel <- struct{}{}
				continue
			}
			if count == 0 {
				outputOrderDeleted(in, false, GetCurrentTimestamp())
				clientChannel <- struct{}{}
				continue
			}
			orderIdToCountMap[in.orderId] = 0
			outputOrderDeleted(in, true, GetCurrentTimestamp())
			clientChannel <- struct{}{}

		default:
			fmt.Fprintf(os.Stderr, "Got order: %c %v x %v @ %v ID: %v\n",
				in.orderType, in.instrument, in.count, in.price, in.orderId)
			outputOrderAdded(in, GetCurrentTimestamp())
			clientChannel <- struct{}{}
		}

	}
}

var globalTimestamp int64 = 0

func (e *Engine) accept(ctx context.Context, conn net.Conn) {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	go e.handleConn(conn)
}

func (e *Engine) handleConn(conn net.Conn) {
	defer conn.Close()
	clientChannel := make(chan struct{})

	for {
		in, err := readInput(conn)
		if err != nil {
			if err != io.EOF {
				_, _ = fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			}
			return
		}
		wrapper := Wrapper{doneChannel: clientChannel, orderInput: in}
		e.wrapperChannel <- wrapper
		<-clientChannel
	}
}

func GetCurrentTimestamp() int64 {
	return time.Now().UnixNano()
}
