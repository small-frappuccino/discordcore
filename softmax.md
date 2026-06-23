Este documento descreve a mecânica e o ciclo de vida de um token na arquitetura de inferência focada em clusters TPU operando sob JetStream e Pathways. A análise a seguir detalha a progressão estrutural e computacional dos dados, desde a ingestão até a emissão.

### 1. Ingestão Assíncrona e Alocação XLA

A entrada de dados multimodais ocorre de forma contínua através de um **ring buffer** assíncrono. O compilador XLA realiza a pré-alocação do mapa de registradores na SRAM e na HBM antes da execução da primeira instrução.

O processamento inicial paraleliza e integra texto (tokenização discreta), vídeo (convolução 3D via ViT) e áudio (decodificação via USM), convergindo os tensores dinamicamente:


$$H_0 = \text{Asynchronous-Stream}\Big( \text{Tokenize}(X_{\text{text}}) E_{\text{text}} \ \big| \ \text{ViT}_{\text{3D}}(X_{\text{vision}}) W_{\text{vision}} \ \big| \ \text{USM}(X_{\text{audio}}) W_{\text{audio}} \Big)$$

Nesta etapa, o JetStream consulta a Radix Tree em memória em busca de **prefix cache hits**, o que permite omitir o reprocessamento de blocos contextuais idênticos.

### 2. Compressão de Estado Híbrida (SSM) e Continuous Batching

Para mitigar o custo de latência na transferência de janelas de contexto via CXL, o pipeline emprega uma arquitetura híbrida. O histórico profundo da sequência é comprimido em um espaço de estado latente de tamanho fixo $\mathcal{O}(1)$ utilizando Modelos Espaciais de Estado (SSM):


$$h_t = A h_{t-1} + B x_t$$

$$y_t = C h_t + D x_t$$

A integração de novos tokens ocorre por meio de **Continuous Batching**. O processamento adota o **Chunked Pre-Fill**, fatiando e inserindo novos tokens na matriz de execução durante os ciclos de decodificação de outras requisições para maximizar a utilização da MXU.

### 3. Estabilização e Projeção com Dynamic Micro-Scaling

A estabilização primária do tensor é executada via RMSNorm:


$$H_{\text{norm}} = \frac{H}{\sqrt{\frac{1}{d} \sum_{i=1}^d h_i^2 + \epsilon}} \odot \gamma$$

As projeções de Consultas ($Q$), Chaves ($K$) e Valores ($V$) operam sob o padrão GQA (Grouped-Query Attention). Em substituição à quantização estática, os **Pallas Kernels** aplicam **Dynamic Micro-Scaling** (FP4 ou MX4). Fatores de escala de ponto flutuante são compartilhados por sub-blocos vetoriais na MXU, o que preserva a precisão de ativações atípicas (outliers) sem saturar o barramento.

A geometria posicional é aplicada via RoPE (Rotary Positional Embedding):


$$q_m = R_{\Theta, m}^d q, \quad k_n = R_{\Theta, n}^d k$$

### 4. Tiling e Online Softmax na SRAM Local

O cálculo de atenção nas camadas Transformer ocorre em uma janela local deslizante. Para manter o processamento estritamente dentro dos limites de memória da SRAM, a sequência é fatiada através de **tiling**.

O processamento utiliza o algoritmo **Online Softmax** para atualizar os valores máximos locais ($m_i$) e os fatores de escala ($l_i$) de forma incremental nos registradores. Isso assegura uma complexidade de memória linear $\mathcal{O}(N)$:


$$m_i = \max(m_{i-1}, \max(x_i))$$

$$l_i = l_{i-1} e^{m_{i-1} - m_i} + \sum e^{x_i - m_i}$$

$$\text{Attention}_{\text{local}} = \frac{e^{x_i - m_i}}{l_i} V_i$$

### 5. Ring Attention Inter-Chip (ICI)

Quando a dimensão da janela de contexto excede a capacidade da memória de um único nó, o **Ring Attention** é ativado. As consultas ($Q$) permanecem ancoradas na SRAM local, enquanto as chaves ($K$) e valores ($V$) são transmitidos fisicamente através do anel da topologia do cluster. O motor CAE (Collectives Acceleration Engine) processa essa comunicação de rede assincronamente em paralelo às operações da MXU:


$$\text{Attention}(Q, K, V) = \text{Softmax}\left(\frac{Q K^T}{\sqrt{d_k}}\right)V$$

### 6. Roteamento Invertido: Expert-Choice MoE

A arquitetura MoE adota o modelo **Expert-Choice Routing**. Em vez de os tokens consultarem os especialistas, a rede projeta as probabilidades espacialmente, permitindo que cada especialista recupere o número exato de tokens exigido pelo seu **Capacity Factor**. Isso garante ocupação contígua no hardware:


$$I_{\text{expert}} = \text{TopK}_{\text{tokens}}\Big( \text{Softmax}(X W_g) \Big)$$

Os tokens roteados são submetidos à transformação não-linear paramétrica SwiGLU:


$$\text{SwiGLU}(x) = \Big( x W_{\text{gate}} \cdot \text{sigmoid}(x W_{\text{gate}}) \Big) (x W_{\text{up}})$$

### 7. Validação Causal Simultânea (Tree Attention)

No contexto de decodificação especulativa, um **Draft Model** gera simultaneamente árvores compostas de 5 a 8 tokens futuros. O modelo principal valida essa estrutura em uma única **forward pass**.

Uma máscara causal bidimensional ($M_{\text{tree}}$) é aplicada para isolar o cálculo de atenção, evitando dependências incorretas entre ramificações especulativas divergentes:


$$\text{Tree Attention}(Q, K, V) = \text{Softmax}\left(\frac{Q K^T}{\sqrt{d_k}} + M_{\text{tree}}\right) V$$

### 8. Descompressão, Amostragem e Emissão (SSE)

O vetor validado é convertido de volta para o domínio do vocabulário ($W_U$), produzindo os logits de saída ($z$). A variação de entropia da distribuição é controlada pela aplicação do parâmetro de Temperatura ($T$):


$$P(y_i) = \frac{\exp(z_i / T)}{\sum_{j=1}^V \exp(z_j / T)}$$

A amostragem estocástica corta a distribuição secundária utilizando **Nucleus Sampling** (Top-p) e Top-k. O token amostrado é enviado imediatamente ao cliente via fluxo **SSE (Server-Sent Events)**.

Concorrentemente, a camada **PagedAttention** consolida os tensores $K$ e $V$ atualizados em memória (ou transmite o estado contínuo para o módulo SSM) e libera os registradores para o próximo ciclo de execução.