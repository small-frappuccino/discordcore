O sistema opera sob um paradigma de decodificação distribuída assíncrona, maximizando a ocupação vetorial da `MXU` e minimizando a saturação do barramento através de particionamento estrito na `SRAM` e compressão latente espacial. A progressão do ciclo de vida do token executa sob determinismo de hardware absoluto, delineado nas etapas estruturais a seguir.

### 1. Ingestão e Alocação Determinística (`XLA`)

A convergência de dados multimodais flui por um `ring buffer` assíncrono, operando como vias de tráfego de alta capacidade desaguando em um único coletor vetorial. O compilador `XLA` exige alocação rígida, mapeando registradores na `SRAM` e limites de paginação na `HBM` antes do disparo do primeiro ciclo de clock. Tensores textuais discretos, matrizes de convolução de vídeo (`ViT`) e decodificadores de áudio (`USM`) são integrados espacialmente:

$$H_0 = \text{Asynchronous-Stream}\Big( \text{Tokenize}(X_{\text{text}}) E_{\text{text}} \ \big| \ \text{ViT}_{\text{3D}}(X_{\text{vision}}) W_{\text{vision}} \ \big| \ \text{USM}(X_{\text{audio}}) W_{\text{audio}} \Big)$$

A latência de inicialização é estritamente suprimida pelo `JetStream`, que interroga uma `Radix Tree` em memória primária para `prefix cache hits`. Isso isola os blocos contextuais idênticos e bloqueia qualquer reprocessamento redundante de estados já mapeados.

### 2. Compressão Espacial e `Continuous Batching`

Para anular o estrangulamento do barramento `CXL` durante transferências massivas de janelas de contexto, o pipeline comprime o histórico profundo em um espaço de estado $\mathcal{O}(1)$ latente, governado por `SSM`. A complexidade temporal e espacial é ancorada de forma isolada:

$$h_t = A h_{t-1} + B x_t$$

$$y_t = C h_t + D x_t$$

A injeção de estados subsequentes processa via `Continuous Batching`. O algoritmo aciona o `Chunked Pre-Fill`, particionando novos tokens e sobrepondo-os aos ciclos ociosos das matrizes de decodificação concorrentes, saturando a capacidade de execução da `MXU`.

### 3. Estabilização e Geometria de Escala

A normalização termodinâmica do tensor opera no limite do ciclo através de `RMSNorm`:

$$H_{\text{norm}} = \frac{H}{\sqrt{\frac{1}{d} \sum_{i=1}^d h_i^2 + \epsilon}} \odot \gamma$$

As projeções topológicas de $Q$, $K$ e $V$ executam sob o particionamento `GQA`. Rejeitando a degradação estrutural da quantização estática, a camada delega aos `Pallas Kernels` a execução do `Dynamic Micro-Scaling` (`FP4` ou `MX4`). Fatores flutuantes calibram sub-blocos independentes na `MXU`, absorvendo *outliers* de ativação sem perfurar a estabilidade do hardware. O mapeamento rotacional posicional é forçado estritamente via `RoPE`:

$$q_m = R_{\Theta, m}^d q, \quad k_n = R_{\Theta, n}^d k$$

### 4. Particionamento Local e `Online Softmax`

O confinamento da mecânica de atenção aos limites físicos da `SRAM` exige o fracionamento em grade via `tiling`. Para assegurar estabilidade iterativa sem desencadear alocações quadráticas massivas, a estrutura emprega o `Online Softmax`. O estado avança atualizando acumuladores de máxima ($m_i$) e fatores exponenciais ($l_i$) nos registradores, ancorando a operação em complexidade de memória $\mathcal{O}(N)$:

$$m_i = \max(m_{i-1}, \max(x_i))$$

$$l_i = l_{i-1} e^{m_{i-1} - m_i} + \sum e^{x_i - m_i}$$

$$\text{Attention}_{\text{local}} = \frac{e^{x_i - m_i}}{l_i} V_i$$

### 5. Sincronização Topológica em Anel (`ICI`)

Quando a matriz escalar rompe os limites de alocação da `SRAM` de um único chip, a malha de comutação engata o `Ring Attention`. As consultas de estado ($Q$) permanecem imutáveis localmente, enquanto as variáveis $K$ e $V$ circulam ao longo do anel físico da rede via `ICI`. O co-processador `CAE` absorve a latência de trânsito em segundo plano de modo estritamente assíncrono:

$$\text{Attention}(Q, K, V) = \text{Softmax}\left(\frac{Q K^T}{\sqrt{d_k}}\right)V$$

### 6. Roteamento de Ocupação (`Expert-Choice MoE`)

O sistema erradica ineficiências estocásticas adotando o roteamento `Expert-Choice MoE`. Especialistas funcionam como coletores independentes, preenchendo seu `Capacity Factor` físico a partir de projeções de probabilidade de token espacial. Isso fixa a ocupação de rotina sem vazamentos de bloco:

$$I_{\text{expert}} = \text{TopK}_{\text{tokens}}\Big( \text{Softmax}(X W_g) \Big)$$

A não-linearidade vetorial atravessa as comportas multiplicativas da `SwiGLU`:

$$\text{SwiGLU}(x) = \Big( x W_{\text{gate}} \cdot \text{sigmoid}(x W_{\text{gate}}) \Big) (x W_{\text{up}})$$

### 7. Verificação Especulativa e `Tree Attention`

Compactando a latência de geração sequencial, o `Draft Model` projeta antecipadamente estruturas arbóreas com 5 a 8 ramificações especulativas. A topologia principal processa as validações em uma única `forward pass`. Uma máscara causal bidimensional estrita ($M_{\text{tree}}$) oblitera dependências interconectadas defeituosas:

$$\text{Tree Attention}(Q, K, V) = \text{Softmax}\left(\frac{Q K^T}{\sqrt{d_k}} + M_{\text{tree}}\right) V$$

### 8. Emissão Assíncrona e Compactação de Estado

Os coeficientes validados retornam ao domínio discreto de vocabulário. A modulação de variação térmica ($T$) calibra a entropia bruta, que é então filtrada pelas restrições nucleares limitantes de `Top-p` e `Top-k`:

$$P(y_i) = \frac{\exp(z_i / T)}{\sum_{j=1}^V \exp(z_j / T)}$$

O fluxo vetorial final é injetado diretamente na rota de escape via protocolo assíncrono `SSE`. Em tempo real, o gerenciador estrito de memória `PagedAttention` trava e arquiva os ponteiros dos tensores na `KV cache`, limpa os sinalizadores dos registradores, e libera os ciclos de memória subsequentes para a próxima iteração contígua do pipeline.