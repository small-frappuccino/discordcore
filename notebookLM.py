import sys
from pathlib import Path

def generate_topology(directory: Path, prefix: str = "") -> str:
    """
    Recursively builds a deterministic directory tree visualization.
    Executes an O(N) traversal. Output is aggregated in RAM before return.
    """
    tree_buffer = []
    # Enforce deterministic ordering: directories first, then files, alphabetically sorted
    paths = sorted(directory.iterdir(), key=lambda p: (not p.is_dir(), p.name))
    pointers = [('├── ', '│   ')] * (len(paths) - 1) + [('└── ', '    ')] if paths else []
    
    for pointer, path in zip(pointers, paths):
        tree_buffer.append(f"{prefix}{pointer[0]}{path.name}\n")
        if path.is_dir():
            tree_buffer.append(generate_topology(path, prefix + pointer[1]))
            
    return "".join(tree_buffer)

def execute_payload_aggregation(base_dir: str = "./pkg"):
    pkg_path = Path(base_dir).resolve()
    
    if not pkg_path.exists() or not pkg_path.is_dir():
        print(f"[FATAL] Target pipeline boundary not found: {pkg_path}")
        sys.exit(1)

    repo_root = pkg_path.parent
    
    # Provision the centralized persistent artifact sink
    output_sink = repo_root / "notebookLM"
    output_sink.mkdir(parents=True, exist_ok=True)
    print(f"[-] Output sink established at: {output_sink}")

    # =================================================================
    # PHASE 1: Root Architectural Ingestion
    # =================================================================
    print("[-] Initializing root architectural aggregation...")
    root_target_files = ["ARCHITECTURE.md", "AGENTS.md"]
    found_root_files = [f for f in root_target_files if (repo_root / f).exists()]

    if found_root_files:
        root_payload_path = output_sink / "ROOT_architecture_payload.md"
        # Initialize memory block
        root_buffer = ["# Root System Architecture & Agent Schemas\n\n"]
        
        for filename in found_root_files:
            file_path = repo_root / filename
            root_buffer.append(f"// === FILE: {filename} ===\n```markdown\n")
            try:
                root_buffer.append(file_path.read_text(encoding="utf-8"))
            except Exception as e:
                root_buffer.append(f"// [I/O FAULT]: Failed to map memory boundary - {e}\n")
            root_buffer.append("\n```\n\n")
            
        # Atomic flush from RAM to persistent disk
        root_payload_path.write_text("".join(root_buffer), encoding="utf-8")
        print(f"[+] Root payload routed to sink: {root_payload_path.name}")
    else:
        print("[!] Target root documents not found. Skipping Phase 1.")

    # =================================================================
    # PHASE 2: Domain Boundary Ingestion (./pkg/*)
    # =================================================================
    domains = [d for d in pkg_path.iterdir() if d.is_dir()]

    for domain in domains:
        print(f"[-] Compiling domain boundary: {domain.name}")
        payload_path = output_sink / f"{domain.name}_notebook_payload.md"
        
        # Initialize contiguous memory buffer for this domain
        domain_buffer = [
            f"# Domain Architecture: {domain.name}\n\n",
            "## Layout Topology\n```text\n",
            f"{domain.name}/\n",
            generate_topology(domain),
            "```\n\n",
            "## Source Stream Aggregation\n\n"
        ]
        
        # Sequential file ingestion into RAM
        go_files = sorted(domain.rglob("*.go"))
        if not go_files:
            domain_buffer.append("> [WARN] 0x00 source streams detected within this layout boundary.\n")
            # Flush empty state to disk
            payload_path.write_text("".join(domain_buffer), encoding="utf-8")
            continue
        
        for go_file in go_files:
            relative_path = go_file.relative_to(repo_root).as_posix()
            
            domain_buffer.append(f"// === FILE: {relative_path} ===\n```go\n")
            try:
                domain_buffer.append(go_file.read_text(encoding="utf-8"))
            except Exception as e:
                domain_buffer.append(f"// [I/O FAULT]: Failed to map memory boundary - {e}\n")
            domain_buffer.append("\n```\n\n")
            
        # Atomic flush from RAM to persistent disk
        payload_path.write_text("".join(domain_buffer), encoding="utf-8")
        print(f"[+] Domain execution complete. Payload routed to sink: {payload_path.name}")

if __name__ == "__main__":
    execute_payload_aggregation()