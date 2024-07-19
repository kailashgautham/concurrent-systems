# Concurrent Systems

Welcome to the ConcurrentSystems repository. This coursework project demonstrates various implementations of high-performance and concurrent systems in C++, Go, and Rust.

## Table of Contents

- [Project Overview](#project-overview)
- [Technologies Used](#technologies-used)
- [Features](#features)
- [Getting Started](#getting-started)
- [Usage](#usage)
- [Contributors](#contributors)

## Project Overview

ConcurrentSystems is a coursework project designed to showcase the development and optimization of concurrent systems across different programming languages. The project includes the following components:

- **C++**: An efficient exchange matching engine designed to handle high volumes of trade orders with optimized execution.
- **Go**: A concurrent exchange matching engine leveraging channels and goroutines to achieve instrument-level concurrency.
- **Rust**: A multi-client TCP server built with Tokio for asynchronous task management, demonstrating significant performance improvements.

## Technologies Used

- **C++**: Standard Library, multithreading
- **Go**: Goroutines, channels
- **Rust**: Tokio, asynchronous programming

## Features

### C++

- High-performance exchange matching engine
- Handles up to 100 threads and 4 million orders
- Optimized trade matching and execution

### Go

- Concurrent exchange matching engine
- Utilizes channels and goroutines for efficient order processing
- Achieves instrument-level concurrency

### Rust

- Multi-client TCP server
- Asynchronous task management with Tokio
- Significant speedup from 58 seconds to 2 seconds

## Getting Started

To get started with this project, clone the repository and follow the instructions for each language-specific implementation.

### Prerequisites

- **C++**: GCC or Clang compiler
- **Go**: Go programming environment
- **Rust**: Rust programming environment

### Installation

1. Clone the repository:
    ```sh
    git clone https://github.com/concurrentsystems/concurrentsystems.git
    ```
2. Navigate to the project directory:
    ```sh
    cd concurrentsystems
    ```

## Usage

### C++

1. Compile the C++ code:
    ```sh
    g++ -std=c++17 -pthread -o exchange_matching_engine_cpp exchange_matching_engine.cpp
    ```
2. Run the executable:
    ```sh
    ./exchange_matching_engine_cpp
    ```

### Go

1. Navigate to the Go project directory:
    ```sh
    cd go_exchange_matching_engine
    ```
2. Run the Go program:
    ```sh
    go run main.go
    ```

### Rust

1. Navigate to the Rust project directory:
    ```sh
    cd rust_tcp_server
    ```
2. Build and run the Rust project:
    ```sh
    cargo run
    ```

## Contributors

- [Gautham Kailash](https://github.com/kailashgautham)
- [Khoo Wui Hong](https://github.com/wui-hong)
