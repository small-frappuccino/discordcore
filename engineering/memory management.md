1. State Decompression: The Quantization Boundary

Before any computation occurs, the system must reconstruct the neural weights ($W$) from their compressed state in memory into a format the execution units can multiply.

$$W \approx W_q \times \alpha$$

$W_q$: The quantized weights (e.g., INT4), stored densely in HBM.

$\alpha$: The micro-scaling vector.

The Mechanics: The hardware reads a highly compressed, memory-efficient matrix, and multiplies it by $\alpha$ at the register level to instantly "inflate" it back into a floating-point geometry right before it hits the multiplier arrays.

2. The Geometric Projection: The MXU Pass

The core of the Transformer is mapping a token's current state ($X$) into the attention space. The Systolic Array executes this as a massive linear algebraic projection.

$$Q, K, V = X \cdot W_{q,k,v}^T + b$$

$X$: The token's current vector representation.

$W^T$: The transposed weight matrix.

The Mechanics: The input is projected into three distinct vectors: a Query ($Q$, what the token is looking for), a Key ($K$, what the token contains), and a Value ($V$, the actual semantic payload).

3. Memory-Bound Fusion: Tiled FlashAttention

Standard attention explodes memory. To keep the $Q, K, V$ matrices locked inside the SRAM without spilling, the math is rewritten to track running maximums, processing the sequence in blocks (tiles).

$$m_j = \max(m_{j-1}, x_j)$$

$$l_j = l_{j-1} e^{m_{j-1} - m_j} + e^{x_j - m_j}$$

$m_j$: The running maximum logit in the block.

$l_j$: The running exponential sum (the denominator of the Softmax).

The Mechanics: Instead of loading the entire sequence geometry into memory at once, the hardware updates a local running tally. It algebraically fends off numerical overflow while computing the exact same attention distribution.

4. Vector Routing: Mixture of Experts (MoE)

When the model is too large for one chip, the network routes the tensor to specific specialized neural blocks (Experts) distributed across the cluster.

$$G(x) = \text{Softmax}(W_g x)$$

$$y = \sum_{i=1}^k G(x)_i \cdot E_i(x)$$

$G(x)$: The Gating probability (which expert gets the token).

$E_i(x)$: The output of that specific Expert network.

The Mechanics: A small routing matrix evaluates the token and generates a probability distribution. The top $k$ probabilities act as physical network switches, directing the data exclusively to the nodes that specialize in that specific semantic geometry.

5. Structural Stabilization: RMSNorm

Repeated matrix multiplications cause tensor values to drift, either decaying to zero or exploding to infinity. The Vector Processing Unit (VPU) forces the geometry back into a stable sphere.

$$\text{RMS}(a) = \sqrt{\frac{1}{d}\sum_{i=1}^{d} a_i^2 + \epsilon}$$

$$y = \frac{a}{\text{RMS}(a)} \odot \gamma$$

$a$: The raw hidden state vector.

$\gamma$: A learned scaling parameter.

The Mechanics: The hardware calculates the variance of the vector, divides the vector by that variance to normalize it, and scales it. This ensures the signal maintains a stable thermodynamic profile as it passes to the next layer.

6. Semantic Interception: CFG & Masking

Before the model makes its final prediction, we mathematically steer it away from unwanted trajectories (like system constraint violations or repeating words).

$$z_{CFG} = z_{uncond} + \gamma (z_{cond} - z_{uncond})$$

$$z_i' = z_{CFG, i} - (\theta \cdot c_i)$$

$z_{cond}, z_{uncond}$: The logits generated with and without the user's explicit constraints.

$\theta \cdot c_i$: The repetition penalty multiplied by how many times the token $c$ has appeared.

The Mechanics: The system calculates the geometric delta between "what the model wants to say" and "what the constraints dictate," physically pushing the matrix away from forbidden semantic spaces.

7. The Probability Collapse: Sampling

The final tensor ($z$) is a vector of raw energy values (logits) mapped to the vocabulary array. To pick the next word, this energy must collapse into a probabilistic curve.

$$P(y_i) = \frac{\exp(z_i / T)}{\sum_{j=1}^V \exp(z_j / T)}$$

$z_i$: The final adjusted logit for word $i$.

$T$: Temperature.

The Mechanics: By exponentiating the logits, the hardware forces all values to be positive. Dividing by the sum of all values normalizes them into a strict $0.0$ to $1.0$ probability. A higher $T$ flattens the curve (more chaos/creativity), while a lower $T$ sharpens it (more deterministic).