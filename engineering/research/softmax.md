O ciclo completo de um token através do hardware mapeia-se na seguinte sequência matemática e física:

1. **Ativação e Carga (`HBM` $\to$ `SRAM`):** Os pesos quantizados em low-precision ingressam na `SRAM` via calibração dinâmica por bloco usando a escala $\alpha$.

$$X_q = \text{round}\left( \frac{X}{\alpha} \right) \times 2^E$$


2. **Projeções de Atenção (`MXU`):** Multiplicação de matrizes para gerar os vetores $Q$, $K$ e $V$. *Aqui ocorre a primeira alteração pelo CFG:* o `MXU` processa em paralelo (ou em batches concatenados) o fluxo condicionado ($X_{cond}$) e o não condicionado ($X_{uncond}$), gerando projeções distintas para ambos.

$$Y = \text{Accumulator}(X_q W_q^T) \times \alpha_X \alpha_W + \beta$$


3. **Fusão de Memória (`Tiled Attention` em `SRAM`):** Redução local usando `Online Softmax` de forma assíncrona para computar as matrizes de atenção sem estourar o limite térmico ou de capacidade da `SRAM`. Se o CFG estiver ativo, o pipeline gerencia dois contextos de `KV Cache` estruturados por blocos.

$$m_j = \max(m_{j-1}, x_j)$$


$$l_j = l_{j-1} e^{m_{j-1} - m_j} + e^{x_j - m_j}$$


$$\text{Out}_j = \frac{l_{j-1} e^{m_{j-1} - m_j} \text{Out}_{j-1} + e^{x_j - m_j} V_j}{l_j}$$


4. **Roteamento de Especialistas (`MoE` via `ICI`):** Onde os tokens passam por camadas esparsas distribuídas entre múltiplos aceleradores físicos conectados pela rede de interconexão.

$$G(x) = \text{Softmax}(W_g x)$$


$$y_i = \sum_{k=1}^K G(x_i)_k \cdot \text{Expert}_k(x_i)$$


5. **Estabilização Vetorial (`VPU`):** O vetor resultante do bloco Transformer passa pelo `RMSNorm` para normalização de variância elemento a elemento, gerando o vetor final de representação oculta ($a$).

$$\text{RMS}(a) = \sqrt{\frac{1}{d}\sum_{i=1}^{d} a_i^2 + \epsilon}$$


$$y = \frac{a}{\text{RMS}(a)} \odot \gamma$$


6. **Interceptação e Penalização Semântica (`VPU` - O Elo Ausente):** O vetor $y$ é projetado contra a matriz de embedding de saída (`LM Head`), gerando os logits puros ($z_{cond}$ e $z_{uncond}$). Antes de enviar esses valores para amostragem estatística, o `VPU` intercepta o fluxo e aplica as penalidades em sequência restrita:
* **Passo A (Divergência Contraste):** Isola a intenção geométrica do usuário.

$$z_{CFG} = z_{uncond} + \gamma (z_{cond} - z_{uncond})$$


* **Passo B (Punição de Frequência e Presença):** Subtrai penalidades com base no histórico de tokens $c_i$ armazenados no estado da aplicação.

$$z_i' = z_{CFG, i} - (\theta \cdot c_i)$$


* **Passo C (Mascaramento Hard):** Se regras gramaticais ou restrições booleanas do sistema forem violadas, zera-se a chance eliminando o logit.

$$z_i' = -\infty \quad \text{se } i \in \text{Estado Violado}$$




7. **Amostragem Discreta (`VPU` $\to$ Host):** Finalmente, o vetor totalmente modificado e retificado $z_i'$ alimenta o cálculo de probabilidade com temperatura ($T$) e corte `Top-p`.

$$P(y_i) = \frac{\exp(z_i' / T)}{\sum_{j=1}^V \exp(z_j' / T)}$$



Abaixo, utilize o simulador interativo para inspecionar visualmente como cada uma dessas fórmulas e restrições manipula a distribuição de energia dos logits em tempo de execução dentro dos registradores da `SRAM`.