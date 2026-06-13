# High-Performance Vector Database Proxy (VectorProxy)

VectorProxy is a specialized high-performance middleware proxy designed to sit transparently between client applications and backend vector databases (such as Qdrant, Milvus, or Pinecone). 

Built in Go, it leverages a modular, synchronous interceptor pipeline to augment queries, manage lifecycles, and optimize database throughput through semantic intelligence.

---

## 🎯 Core Goals

*   **Decouple Applications from Embedding APIs**: Automatically generate embeddings in flight, freeing the client from managing embedding model calls.
*   **Optimize Database Throughput**: Mitigate database search load via micro-batching, in-flight coalescing, and semantic caching.
*   **Intelligent Request Routing**: Inspect queries semantically and dispatch them to the specialized database index and embedding model that best fits the query intent.

---

## 🗺️ Feature Roadmap & Implementation Status

Below is the implementation status of the core proxy features:

- [x] **gRPC Server Layer & Reflection**: High-throughput, low-latency communication layer defined in [proxy.proto](file:///home/ihgazi/Projects/VectorProxy/proto/proxy/v1/proxy.proto).
- [x] **Modular Middleware Pipeline**: Chained synchronous interceptors configured dynamically via [pipeline.go](file:///home/ihgazi/Projects/VectorProxy/internal/middleware/pipeline.go).
- [x] **In-Flight Embedding Generation**: Intercepts text-only queries and automatically populates float vectors using the local [OllamaEmbedder](file:///home/ihgazi/Projects/VectorProxy/internal/embedding/ollama.go).
- [x] **Qdrant Vector Store Integration**: Implements the backend [VectorStore](file:///home/ihgazi/Projects/VectorProxy/internal/engine/interface.go) interface to translate filters and query vectors for Qdrant.
- [x] **Latency & Performance Logging**: Basic diagnostic tracking built directly into the interceptor pipeline.
- [x] **Probabilistic Semantic Caching**: Using vector similarity checks against low-latency stores (e.g., Redis-VL) to bypass backend databases for semantically duplicate queries.
- [x] **Request Coalescing (Collapsing)**: Merges identical concurrent search requests in-flight to prevent the thundering herd problem.
- [x] **Vectorized Micro-Batching**: Accumulates incoming individual searches over a short window (e.g., 10–50ms) to dispatch them as a single multi-vector batch query.
- [ ] **Write/Update Support (CRUD)**: Adding Upsert, Delete, and Collection management endpoints to make the proxy fully transparent.

---

## 🏃 Getting Started

### Prerequisites
*   Go (version `1.26.2` or later)
*   [Ollama](https://ollama.com/) running locally with the `nomic-embed-text` model pulled:
    ```bash
    ollama pull nomic-embed-text
    ```
*   A Qdrant vector database running on `localhost:6334` (gRPC port).


### Running the Proxy
Start the proxy on `:50051`:
```bash
go run cmd/proxy/main.go
```
