use std::sync::mpsc;
use std::error::Error;
use tokio::net::{TcpListener, TcpStream};
use tokio::io::{BufReader, AsyncBufReadExt, AsyncWriteExt};
use crate::task::Task;
use crate::task::TaskType;
use std::sync::Arc;
use tokio::sync::{Semaphore};

pub trait ServerTrait {
    fn start_server(
        &self,
        address: String,
        tx: mpsc::Sender<Result<(), Box<dyn Error + Send>>>,
    );
}

pub struct Server;

impl ServerTrait for Server {

    #[tokio::main]
    async fn start_server(
        &self,
        address: String,
        tx: mpsc::Sender<Result<(), Box<dyn Error + Send>>>,
    ) {
            let sem = Arc::new(Semaphore::new(40));
            println!("Starting the server");
            let listener = TcpListener::bind(address).await;
    
            let listener = match listener {
                Ok(listener) => {
                    tx.send(Ok(())).unwrap();
                    listener
                },
                Err(e) => {
                    println!("here {}", e);
                    tx.send(Err(Box::new(e))).unwrap();
                    return;
                }
            };
            loop {
                match listener.accept().await {
                    Ok((stream, _)) => {
                        let clone = sem.clone();
                        tokio::spawn(async move {
                            Self::handle_connection(stream, clone).await;
                        });
                    },
                    Err(e) => {
                        eprintln!("Error accepting connection: {}", e);
                    }
                }
            }
    }
}

impl Server {
    async fn handle_connection(mut stream: TcpStream, sem: Arc<Semaphore>) {
        //split reader and writer to avoid multiple mutable borrow
        let (reader, mut writer) = stream.split();
        let reader = BufReader::new(reader);
        let mut lines = reader.lines();

        while let Ok(Some(line)) = lines.next_line().await {
            let clone = sem.clone();
            let response = Self::get_task_value(line, clone).await;
            if let Some(r) = response {
                if let Err(e) = writer.write_all(&[r]).await {
                    eprintln!("Failed to write response to stream: {}", e);
                    return;
                }
            }
        }
    }

    async fn get_task_value(buf: String, sem: Arc<Semaphore>) -> Option<u8> {
        let numbers: Vec<&str> = buf.trim().split(':').collect();
        let task_type = numbers.first().unwrap().parse::<u8>().ok()?;
        let seed = numbers.last().unwrap().parse::<u64>().ok()?;
        if TaskType::from_u8(task_type)? == TaskType::CpuIntensiveTask {
            let _permit = sem.acquire().await.unwrap();
            Some(Task::execute_async(task_type, seed).await)
        } else {
            Some(Task::execute_async(task_type, seed).await)
        } 
    }
}