#include <iostream>
#include <thread>
#include <atomic>
#include <queue>
#include <map>
#include <unordered_map>
#include <memory>
#include <mutex>
#include "io.hpp"
#include "engine.hpp"

struct order {
    char input_type;
    uint32_t order_id;
    uint32_t price;
    uint32_t count;
	uint32_t execution_id;
    std::string instrument;
    unsigned long long timestamp;
};

struct instrument_struct {
	std::mutex instrument_mutex;
	std::map<uint32_t, std::queue<order>> instrument_buy_map;
	std::map<uint32_t, std::queue<order>> instrument_sell_map;
};

std::unordered_map<std::string, instrument_struct> instrument_to_instrument_struct;

std::unordered_map<uint32_t, order> order_id_map;

std::unordered_map<std::string, std::unordered_map<uint32_t, uint32_t>> instrument_to_id_map;

std::mutex orderbook_mutex;
std::mutex order_id_map_mutex;

std::atomic_ullong global_timestamp = 0;

void Engine::accept(ClientConnection connection)
{
	auto thread = std::thread(&Engine::connection_thread, this, std::move(connection));
	thread.detach();
}

void Engine::connection_thread(ClientConnection connection)
{
	while(true)
	{
		ClientCommand input {};
		switch(connection.readInput(input))
		{
			case ReadResult::Error: SyncCerr {} << "Error reading input" << std::endl;
			case ReadResult::EndOfFile: return;
			case ReadResult::Success: break;
		}

		switch(input.type)
		{
			case input_buy: {

				order active_buy = {
					input_buy,
					input.order_id,
					input.price,
					input.count,
					1,
					input.instrument,
					global_timestamp.fetch_add(1),
				};
				
				uint32_t count_to_satisfy = input.count;
				std::unique_lock<std::mutex> orderbook_lock(orderbook_mutex);
				instrument_struct& curr_struct = instrument_to_instrument_struct[input.instrument];
				auto& instrument_map = instrument_to_id_map[input.instrument];
				orderbook_lock.unlock();
				std::unique_lock<std::mutex> instrument_lock(curr_struct.instrument_mutex);

				//no sell orders exist :(
				if (curr_struct.instrument_sell_map.size() == 0) {
					Output::OrderAdded(
						active_buy.order_id, 
						active_buy.instrument.c_str(), 
						active_buy.price, 
						active_buy.count, 
						false,
						global_timestamp.fetch_add(1)
					);
					curr_struct.instrument_buy_map[active_buy.price].push(active_buy);

					std::unique_lock<std::mutex> order_id_map_lock(order_id_map_mutex);
					order_id_map[active_buy.order_id] = active_buy;
					instrument_map[active_buy.order_id] = active_buy.count;
					// uwu unlock orderid mutex
					order_id_map_lock.unlock();
					instrument_lock.unlock();
					break;
				}
				
				//sell greater than buy, which is invalid.
				if (curr_struct.instrument_sell_map.begin()->first > input.price) {
					Output::OrderAdded(
						active_buy.order_id, 
						active_buy.instrument.c_str(), 
						active_buy.price, 
						active_buy.count, 
						false,
						global_timestamp.fetch_add(1)
					);
					curr_struct.instrument_buy_map[active_buy.price].push(active_buy);

					std::unique_lock<std::mutex> order_id_map_lock(order_id_map_mutex);
					order_id_map[active_buy.order_id] = active_buy;
					instrument_map[active_buy.order_id] = active_buy.count;

					order_id_map_lock.unlock();
					instrument_lock.unlock();
					break;
				}

				//the active order is yet to be satisfied
				while (count_to_satisfy > 0) {
					auto& sell_queue = curr_struct.instrument_sell_map.begin()->second;
					order& sell_order = sell_queue.front();
					if (instrument_map[sell_order.order_id] == 0) {
						sell_queue.pop();
						if (sell_queue.empty()) {
							curr_struct.instrument_sell_map.erase(sell_order.price);
							if (curr_struct.instrument_sell_map.empty() || curr_struct.instrument_sell_map.begin()->first > active_buy.price) {
								break;
							}
						}
						continue;
					}
					//found a sell order
					uint32_t amount_sold = std::min(count_to_satisfy, instrument_map[sell_order.order_id]);
					instrument_map[sell_order.order_id] -= amount_sold;
					Output::OrderExecuted(
						sell_order.order_id, 
						active_buy.order_id, 
						sell_order.execution_id++,
						sell_order.price,
						amount_sold,
						global_timestamp.fetch_add(1)
					);

					count_to_satisfy -= amount_sold;

					if (instrument_map[sell_order.order_id] == 0) {
						sell_queue.pop();
					}

					if (sell_queue.empty()) {
						curr_struct.instrument_sell_map.erase(sell_order.price);
						if (curr_struct.instrument_sell_map.empty() || curr_struct.instrument_sell_map.begin()->first > active_buy.price) {
							break;
						}
					}
				}
				
				if (count_to_satisfy != 0) {
					active_buy.count = count_to_satisfy;
					Output::OrderAdded(
						active_buy.order_id, 
						active_buy.instrument.c_str(), 
						active_buy.price, 
						active_buy.count, 
						false,
						global_timestamp.fetch_add(1)
					);

					curr_struct.instrument_buy_map[active_buy.price].push(active_buy);

					std::unique_lock<std::mutex> order_id_map_lock(order_id_map_mutex);
					order_id_map[active_buy.order_id] = active_buy;
					instrument_map[active_buy.order_id] = active_buy.count;
					order_id_map_lock.unlock();
				}
				instrument_lock.unlock();
				break;
			}

			case input_sell: {
				order active_sell = {
					input_sell,
					input.order_id,
					input.price,
					input.count,
					1,
					input.instrument,
					global_timestamp.fetch_add(1),
				};

				uint32_t count_to_satisfy = input.count;
				std::unique_lock<std::mutex> orderbook_lock(orderbook_mutex);
				instrument_struct& curr_struct = instrument_to_instrument_struct[input.instrument];
				auto& instrument_map = instrument_to_id_map[input.instrument];
				orderbook_lock.unlock();
				std::unique_lock<std::mutex> instrument_lock(curr_struct.instrument_mutex);

				//no sell orders exist :(
				if (curr_struct.instrument_buy_map.size() == 0) {
					Output::OrderAdded(
						active_sell.order_id, 
						active_sell.instrument.c_str(), 
						active_sell.price, 
						active_sell.count, 
						true,
						global_timestamp.fetch_add(1)
					);
					curr_struct.instrument_sell_map[active_sell.price].push(active_sell);

					std::unique_lock<std::mutex> order_id_map_lock(order_id_map_mutex);
					order_id_map[active_sell.order_id] = active_sell;
					instrument_map[active_sell.order_id] = active_sell.count;
					order_id_map_lock.unlock();
					instrument_lock.unlock();
					break;
				}
				
				//sell greater than buy, which is invalid.
				if (curr_struct.instrument_buy_map.rbegin()->first < input.price) {
					Output::OrderAdded(
						active_sell.order_id, 
						active_sell.instrument.c_str(), 
						active_sell.price, 
						active_sell.count, 
						true,
						global_timestamp.fetch_add(1)
					);
					curr_struct.instrument_sell_map[active_sell.price].push(active_sell);

					std::unique_lock<std::mutex> order_id_map_lock(order_id_map_mutex);
					order_id_map[active_sell.order_id] = active_sell;
					instrument_map[active_sell.order_id] = active_sell.count;

					order_id_map_lock.unlock();
					instrument_lock.unlock();
					break;
				}

				while (count_to_satisfy > 0) {

					auto& buy_queue = curr_struct.instrument_buy_map.rbegin()->second;
					order& buy_order = buy_queue.front();
					if (instrument_map[buy_order.order_id] == 0) {
						buy_queue.pop();
						if (buy_queue.empty()) {
							curr_struct.instrument_buy_map.erase(buy_order.price);
							if (curr_struct.instrument_buy_map.empty() || curr_struct.instrument_buy_map.rbegin()->first < active_sell.price) {
								break;
							}
						}
						continue;
					}

					uint32_t amount_sold = std::min(count_to_satisfy, instrument_map[buy_order.order_id]);
					instrument_map[buy_order.order_id] -= amount_sold;
					Output::OrderExecuted(
						buy_order.order_id, 
						active_sell.order_id, 
						buy_order.execution_id++,
						buy_order.price,
						amount_sold,
						global_timestamp.fetch_add(1)
					);

					count_to_satisfy -= amount_sold;

					if (instrument_map[buy_order.order_id] == 0) {
						buy_queue.pop();
					}

					if (buy_queue.empty()) {
						curr_struct.instrument_buy_map.erase(buy_order.price);
						if (curr_struct.instrument_buy_map.empty() || curr_struct.instrument_buy_map.rbegin()->first < active_sell.price) {
							break;
						}
					}
				}

				if (count_to_satisfy != 0) {
					active_sell.count = count_to_satisfy;
					Output::OrderAdded(
						active_sell.order_id, 
						active_sell.instrument.c_str(), 
						active_sell.price, 
						active_sell.count, 
						true,
						global_timestamp.fetch_add(1)
					);

					curr_struct.instrument_sell_map[active_sell.price].push(active_sell);
					
					std::unique_lock<std::mutex> order_id_map_lock(order_id_map_mutex);
					order_id_map[active_sell.order_id] = active_sell;
					instrument_map[active_sell.order_id] = active_sell.count;
					order_id_map_lock.unlock();
				}
				instrument_lock.unlock();
				break;
			}

			case input_cancel: {

				std::unique_lock<std::mutex> order_id_map_lock(order_id_map_mutex);

				//cannot find it inside order_id_map
				if (order_id_map.find(input.order_id) == order_id_map.end()) {
					Output::OrderDeleted(input.order_id, false, global_timestamp.fetch_add(1));
					order_id_map_lock.unlock();
					break;
				}

				//order exists but might have been deleted, instrument definitely exists
				order curr_order = order_id_map[input.order_id];
				order_id_map_lock.unlock();

				std::unique_lock<std::mutex> orderbook_lock(orderbook_mutex);
				//get a reference to the instrument map inside instrument to instrument map
				auto& instrument_map = instrument_to_id_map[curr_order.instrument];
				auto& curr_struct = instrument_to_instrument_struct[curr_order.instrument];
				orderbook_lock.unlock();

				std::unique_lock<std::mutex> instrument_lock(curr_struct.instrument_mutex);

				//order has been deleted already
				if (instrument_map[input.order_id] == 0) {
					Output::OrderDeleted(input.order_id, false, global_timestamp.fetch_add(1));
					instrument_lock.unlock();
					break;
				}

				else {
					instrument_map[input.order_id] = 0;
					Output::OrderDeleted(input.order_id, true, global_timestamp.fetch_add(1));
					instrument_lock.unlock();
					break;
				}
			}

			//Handles invalid input
			default: {
				SyncCerr {}
				    << "Got order: " << static_cast<char>(input.type) << " " << input.instrument << " x " << input.count << " @ "
				    << input.price << " ID: " << input.order_id << std::endl;
				auto output_time = getCurrentTimestamp();
				Output::OrderAdded(input.order_id, input.instrument, input.price, input.count, input.type == input_sell,
				    output_time);
				break;
			}
		}
	}
}
