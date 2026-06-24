import sys
from pathlib import Path

def generate_topology(directory: Path, prefix: str = "") -> str:
    """
    Recursively builds a deterministic directory tree visualization.
    Executes an O(N) traversal where N is the number of nested system entities.
    """
    tree_str = ""
    # Enforce deterministic ordering: directories first, then files, alphabetically sorted
    paths = sorted(directory.iterdir(), key=lambda p: (not p.is_dir(), p.name))
    pointers = [('├── ', '│   ')] * (len(paths) - 1) + [('└── ', '    ')] if paths else []
    
    for pointer, path in zip(pointers, paths):
        # Prevent recursive self-ingestion of the generated payload files
        if path.name.endswith("_notebook_payload.md"):
            continue
            
        tree_str += f"{prefix}{pointer[0]}{path.name}\n"
        if path.is_dir():
            tree_str += generate_topology(path, prefix + pointer[1])
    return tree_str

def execute_payload_aggregation(base_dir: str = "./pkg"):
    pkg_path = Path(base_dir).resolve()
    
    if not pkg_path.exists() or not pkg_path.is_dir():
        print(f"[FATAL] Target pipeline boundary not found: {pkg_path}")
        sys.exit(1)

    repo_root = pkg_path.parent

    # =================================================================
    # PHASE 1: Root Architectural Ingestion
    # =================================================================
    print("[-] Initializing root architectural aggregation...")
    root_target_files = ["ARCHITECTURE.md", "AGENTS.md"]
    found_root_files = [f for f in root_target_files if (repo_root / f).exists()]

    if found_root_files:
        root_payload_path = repo_root / "ROOT_architecture_payload.md"
        with open(root_payload_path, "w", encoding="utf-8") as root_md:
            root_md.write("# Root System Architecture & Agent Schemas\n\n")
            
            for filename in found_root_files:
                file_path = repo_root / filename
                root_md.write(f"// === FILE: {filename} ===\n")
                root_md.write("```markdown\n")
                try:
                    root_md.write(file_path.read_text(encoding="utf-8"))
                except Exception as e:
                    root_md.write(f"// [I/O FAULT]: Failed to map memory boundary - {e}\n")
                root_md.write("\n```\n\n")
        print(f"[+] Root payload compiled at: {root_payload_path}")
    else:
        print("[!] Target root documents not found. Skipping Phase 1.")

    # =================================================================
    # PHASE 2: Domain Boundary Ingestion (./pkg/*)
    # =================================================================
    # Enforce structural strictness: ./pkg/* contains NO root Go files.
    # We only map and iterate through directory objects (Domains).
    domains = [d for d in pkg_path.iterdir() if d.is_dir()]

    for domain in domains:
        print(f"[-] Compiling domain boundary: {domain.name}")
        payload_path = domain / f"{domain.name}_notebook_payload.md"
        
        # Enforce UTF-8 to bypass native Windows character-mapping faults
        with open(payload_path, "w", encoding="utf-8") as md_file:
            # 1. Structural Header & Topology
            md_file.write(f"# Domain Architecture: {domain.name}\n\n")
            md_file.write("## Layout Topology\n```text\n")
            md_file.write(f"{domain.name}/\n")
            md_file.write(generate_topology(domain))
            md_file.write("```\n\n")
            md_file.write("## Source Stream Aggregation\n\n")
            
            # 2. Sequential Asynchronous File Ingestion Simulation
            go_files = sorted(domain.rglob("*.go"))
            if not go_files:
                md_file.write("> [WARN] 0x00 source streams detected within this layout boundary.\n")
                continue
            
            for go_file in go_files:
                # Force POSIX format (forward slashes) on Windows for LLM ingestion consistency
                relative_path = go_file.relative_to(repo_root).as_posix()
                
                md_file.write(f"// === FILE: {relative_path} ===\n")
                md_file.write("```go\n")
                try:
                    md_file.write(go_file.read_text(encoding="utf-8"))
                except Exception as e:
                    md_file.write(f"// [I/O FAULT]: Failed to map memory boundary - {e}\n")
                md_file.write("\n```\n\n")
                
        print(f"[+] Domain execution complete. Payload anchored at: {payload_path}")

if __name__ == "__main__":
    execute_payload_aggregation()