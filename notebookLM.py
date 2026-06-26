import os
import sys
import zipfile
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Callable, Dict, List, Tuple, Set, Optional
import concurrent.futures

# =================================================================
# INVARIANT CONSTANTS & EXCLUSION BOUNDARIES
# =================================================================

EXCLUDED_DIR_PARTS: frozenset[str] = frozenset({
    "vendor", "testdata", "_gen", "test", "misc", ".git",
    "hack", "third_party", ".github"
})

EXCLUDED_FILE_ENDS: frozenset[str] = frozenset({
    "_gen.go", "_test.go", ".tree.txt", ".yaml", ".golden"
})

K8S_STAGING_ALLOWED: frozenset[str] = frozenset({
    "apiserver/pkg/server/genericapiserver.go",
    "apiserver/pkg/endpoints/handlers/fieldmanager",
    "apiserver/pkg/storage/etcd3/store.go",
    "client-go/tools/cache/reflector.go",
    "client-go/tools/cache/delta_fifo.go",
    "client-go/util/workqueue/queue.go",
})

K8S_PKG_ALLOWED: frozenset[str] = frozenset({
    "registry",
    "scheduler/backend/queue/scheduling_queue.go",
    "controller/garbagecollector/garbagecollector.go",
    "kubelet/pleg/pleg.go",
    "kubelet/cm/cpumanager/cpu_manager.go",
})

# =================================================================
# CORE MECHANICS & DISK I/O
# =================================================================

def should_exclude(path: Path) -> bool:
    r"""
    [Mechanic]: Evaluates path topology against strict centralized exclusion rules.
    [Complexity]: \mathcal{O}(K) where K is the depth of the path tree.
    """
    name_lower = path.name.lower()

    for end in EXCLUDED_FILE_ENDS:
        if name_lower.endswith(end):
            return True

    for part in path.parts:
        if part.lower() in EXCLUDED_DIR_PARTS:
            return True

    return False

def generate_topology(directory: Path, prefix: str = "", exclude_fn: Optional[Callable[[Path], bool]] = None) -> str:
    r"""
    [Mechanic]: Recursively executes a deterministic \mathcal{O}(N) directory tree traversal.
    """
    tree_buffer: List[str] = []

    paths = [p for p in directory.iterdir() if exclude_fn is None or not exclude_fn(p)]
    paths.sort(key=lambda p: (not p.is_dir(), p.name))

    pointers = [('├── ', '│   ')] * (len(paths) - 1) + [('└── ', '    ')] if paths else []

    for pointer, path in zip(pointers, paths):
        tree_buffer.append(f"{prefix}{pointer[0]}{path.name}\n")
        if path.is_dir():
            tree_buffer.append(generate_topology(path, prefix + pointer[1], exclude_fn))

    return "".join(tree_buffer)

def convert_odt_to_markdown(odt_path: Path) -> str:
    """
    [Mechanic]: Deserializes OpenDocument Text (.odt) archives into flat Markdown strings.
    """
    ns = {
        'office': 'urn:oasis:names:tc:opendocument:xmlns:office:1.0',
        'text': 'urn:oasis:names:tc:opendocument:xmlns:text:1.0',
        'table': 'urn:oasis:names:tc:opendocument:xmlns:table:1.0',
        'style': 'urn:oasis:names:tc:opendocument:xmlns:style:1.0',
    }

    def extract_text(element: ET.Element) -> str:
        parts = [element.text or ""]
        for child in element:
            parts.append(extract_text(child))
            if child.tail:
                parts.append(child.tail)
        return "".join(parts)

    try:
        with zipfile.ZipFile(odt_path) as z:
            content_xml = z.read("content.xml")
            root = ET.fromstring(content_xml)

            style_parent_map: Dict[str, str] = {}
            auto_styles = root.find('.//office:automatic-styles', ns)
            if auto_styles is not None:
                for style in auto_styles:
                    name = style.attrib.get(f"{{{ns['style']}}}name", "")
                    parent = style.attrib.get(f"{{{ns['style']}}}parent-style-name", "")
                    if name and parent:
                        style_parent_map[name] = parent

            def get_heading_level(style_name: str) -> int:
                if not style_name: return 0
                parent = style_parent_map.get(style_name, style_name)
                if parent == 'Heading_20_1' or parent.startswith('Heading 1') or parent.endswith('1'): return 1
                if parent == 'Heading_20_2' or parent.startswith('Heading 2') or parent.endswith('2'): return 2
                if parent == 'Heading_20_3' or parent.startswith('Heading 3') or parent.endswith('3'): return 3
                if parent == 'Heading_20_4' or parent.startswith('Heading 4') or parent.endswith('4'): return 4
                return 0

            body = root.find('.//office:body', ns)
            if body is None: return ""
            text_body = body.find('.//office:text', ns)
            if text_body is None: return ""

            md_lines: List[str] = []

            for child in text_body:
                tag = child.tag.split('}')[-1]
                if tag == 'p':
                    style_name = child.attrib.get(f"{{{ns['text']}}}style-name", "")
                    level = get_heading_level(style_name)
                    txt = extract_text(child).strip()
                    if txt:
                        md_lines.append(f"{'#' * level} {txt}\n" if level > 0 else f"{txt}\n")
                elif tag == 'list':
                    for item in child.findall('.//text:list-item', ns):
                        for p in item.findall('.//text:p', ns):
                            txt = extract_text(p).strip()
                            if txt: md_lines.append(f"- {txt}")
                    md_lines.append("")
                elif tag == 'table':
                    rows = child.findall('.//table:table-row', ns)
                    if not rows: continue
                    for r_idx, row in enumerate(rows):
                        cells = row.findall('.//table:table-cell', ns)
                        cell_texts = [" ".join(filter(None, [extract_text(p).strip() for p in cell.findall('.//text:p', ns)])) for cell in cells]
                        md_lines.append(f"| {' | '.join(cell_texts)} |")
                        if r_idx == 0:
                            seps = ['---'] * len(cell_texts)
                            md_lines.append(f"| {' | '.join(seps)} |")
                    md_lines.append("")

            return "\n".join(md_lines)
    except Exception as e:
        return f"// [I/O FAULT]: Failed to convert ODT hardware boundary - {e}\n"

# =================================================================
# DOMAIN TOPOLOGY & CLUSTER SPLITTING
# =================================================================

def read_file_safe(file_path: Path) -> str:
    """Isolates exact memory mapping for individual hardware files."""
    try:
        if file_path.suffix.lower() == ".odt":
            return convert_odt_to_markdown(file_path)
        return file_path.read_text(encoding="utf-8")
    except Exception as e:
        return f"// [I/O FAULT]: Memory boundary mapping failed - {e}\n"

def get_heuristic_word_count(file_path: Path) -> int:
    r"""
    [Mechanic]: Bypasses physical disk reads during domain splitting via byte-size heuristics.
    [Optimization]: Eliminates an \mathcal{O}(N) file-read bottleneck across the entire repository.
    """
    try:
        # Standard software heuristic: 1 word roughly equals 5 bytes
        return file_path.stat().st_size // 5
    except OSError:
        return 0

def split_domain(current_dir: Path, files: List[Path], base_domain: Path, word_counts: Dict[Path, int], is_root_call: bool = True, max_words: int = 500000) -> List[Tuple[str, List[Path], Path]]:
    """
    [Mechanic]: Recursively chunks data segments based strictly on hardware token limits.
    """
    total_words = sum(word_counts.get(f, 0) for f in files)

    if total_words <= max_words:
        suffix = "" if is_root_call else "_".join(current_dir.relative_to(base_domain).parts)
        return [(suffix, files, current_dir)]

    subdirs: Set[Path] = set()
    for f in files:
        try:
            rel = f.relative_to(current_dir)
            if len(rel.parts) > 1:
                subdirs.add(current_dir / rel.parts[0])
        except ValueError:
            pass

    if not subdirs:
        suffix = "" if is_root_call else "_".join(current_dir.relative_to(base_domain).parts)
        return [(suffix, files, current_dir)]

    results: List[Tuple[str, List[Path], Path]] = []
    root_files = [f for f in files if f.parent == current_dir]

    if root_files:
        suffix = "root" if is_root_call else "_".join(current_dir.relative_to(base_domain).parts) + "_root"
        results.append((suffix, root_files, current_dir))

    for subdir in sorted(subdirs):
        subdir_files = [f for f in files if (f.is_relative_to(subdir) if hasattr(f, 'is_relative_to') else str(f).startswith(str(subdir)))]
        if subdir_files:
            results.extend(split_domain(subdir, subdir_files, base_domain, word_counts, is_root_call=False, max_words=max_words))

    return results

def is_k8s_allowed(file_path: Path, k8s_pkg_path: Path, k8s_staging_path: Path) -> bool:
    try:
        rel_pkg = file_path.relative_to(k8s_pkg_path).as_posix()
        if any(rel_pkg == allowed or rel_pkg.startswith(allowed + "/") for allowed in K8S_PKG_ALLOWED):
            return True
    except ValueError: pass

    try:
        rel_staging = file_path.relative_to(k8s_staging_path).as_posix()
        if any(rel_staging == allowed or rel_staging.startswith(allowed + "/") for allowed in K8S_STAGING_ALLOWED):
            return True
    except ValueError: pass

    return False

# =================================================================
# ASYNCHRONOUS PIPELINE ORCHESTRATOR
# =================================================================

def process_domain_chunk(chunk_data: Tuple[str, List[Path], Path], domain: Path, is_golang_ref: bool, is_k8s_ref: bool,
                         compile_internal_path: Path, output_sinks: Dict[str, Path], split_parents: Set[str], repo_root: Path, ref_base: Path):
    """
    [Mechanic]: Executes localized domain processing on an isolated thread.
    """
    suffix, chunk_files, chunk_dir = chunk_data

    # 1. Path Resolution
    if is_golang_ref:
        is_compile_internal_sub = (domain.is_relative_to(compile_internal_path) and domain != compile_internal_path)
        if is_compile_internal_sub:
            base_name, arch_title = f"golang_cmd_compile_internal_{domain.name}", f"cmd/compile/internal/{domain.name}"
        else:
            parent_name = domain.parent.name
            if parent_name in split_parents:
                base_name, arch_title = f"golang_{parent_name}_{domain.name}", f"{parent_name}/{domain.name}"
            else:
                base_name, arch_title = f"golang_{domain.name}", domain.name
    elif is_k8s_ref:
        is_staging = "staging" in domain.parts
        if is_staging:
            base_name, arch_title = f"k8s_staging_{domain.name}", f"staging/src/k8s.io/{domain.name}"
        else:
            base_name, arch_title = f"k8s_pkg_{domain.name}", f"pkg/{domain.name}"
    else:
        base_name, arch_title = f"{domain.name}_notebook_payload", domain.name

    payload_name = f"{base_name}_{suffix}.md" if suffix else f"{base_name}.md"
    chunk_arch_title = f"{arch_title} ({suffix})" if suffix else arch_title

    # 2. Sink Routing
    sink_key = "go" if is_golang_ref else "k8s" if is_k8s_ref else "discordcore"
    payload_path = output_sinks[sink_key] / payload_name

    # 3. Direct File-Stream Writing
    with open(payload_path, "w", encoding="utf-8") as out_f:
        out_f.write(f"# Domain Architecture: {chunk_arch_title}\n\n")
        out_f.write("## Layout Topology\n```text\n")
        out_f.write(f"{chunk_arch_title}/\n")

        base_exclude_fn = (lambda p: should_exclude(p) or p.name == "internal") if (is_golang_ref and domain.name == "compile") else should_exclude
        out_f.write(generate_topology(chunk_dir, exclude_fn=base_exclude_fn))
        out_f.write("```\n\n## Source Stream Aggregation\n\n")

        if not chunk_files:
            out_f.write("> [WARN] 0x00 source streams detected within this layout boundary.\n")
            return

        for file_path in chunk_files:
            try: relative_path = file_path.relative_to(repo_root).as_posix()
            except ValueError:
                try: relative_path = (Path("references") / file_path.relative_to(ref_base)).as_posix()
                except ValueError: relative_path = file_path.as_posix()

            content = read_file_safe(file_path)

            if file_path.suffix.lower() == ".odt":
                out_f.write(f"// === FILE: {relative_path.replace('.odt', '.md')} ===\n```markdown\n{content}\n```\n\n")
            else:
                lang = "go" if file_path.suffix.lower() == ".go" else "markdown" if file_path.suffix.lower() == ".md" else "text"
                out_f.write(f"// === FILE: {relative_path} ===\n```{lang}\n{content}\n```\n\n")

    print(f"[+] Domain execution complete. Hardware mapping routed to: {payload_path.name}")

def execute_payload_aggregation(base_dir: str = "./pkg"):
    pkg_path = Path(base_dir).resolve()

    if not pkg_path.exists() or not pkg_path.is_dir():
        print(f"[FATAL] Target pipeline boundary not found: {pkg_path}")
        sys.exit(1)

    repo_root = pkg_path.parent

    # Persistent Artifact Sink Provisioning
    output_sinks = {
        "discordcore": repo_root / "notebookLM" / "discordcore",
        "go": repo_root / "notebookLM" / "go",
        "k8s": repo_root / "notebookLM" / "kubernetes"
    }
    for sink in output_sinks.values():
        sink.mkdir(parents=True, exist_ok=True)
    print(f"[-] Hardware sinks established at: {repo_root / 'notebookLM'}")

    # PHASE 1: Root Architectural Ingestion
    root_target_files = ["ARCHITECTURE.md", "AGENTS.md", "softmax.md"]
    found_root_files = [f for f in root_target_files if (repo_root / f).exists()]

    if found_root_files:
        root_payload_path = output_sinks["discordcore"] / "ROOT_architecture_payload.md"
        with open(root_payload_path, "w", encoding="utf-8") as rf:
            rf.write("# Root System Architecture & Agent Schemas\n\n")
            for filename in found_root_files:
                rf.write(f"// === FILE: {filename} ===\n```markdown\n{read_file_safe(repo_root / filename)}\n```\n\n")
        print(f"[+] Root payload routed to sink: {root_payload_path.name}")

    # PHASE 1.5: Explicit Golang Modules Processing (go.mod, go.sum)
    # Re-routed sink to output_sinks["discordcore"] per system directive
    go_mod_files = ["go.mod", "go.sum"]
    for g_file in go_mod_files:
        g_path = repo_root / g_file
        if g_path.exists():
            mod_payload_path = output_sinks["discordcore"] / f"{g_file.replace('.', '_')}_payload.md"
            with open(mod_payload_path, "w", encoding="utf-8") as mf:
                mf.write(f"# Root Module State: {g_file}\n\n")
                mf.write(f"// === FILE: {g_file} ===\n```go\n{read_file_safe(g_path)}\n```\n\n")
            print(f"[+] Module state isolated to discordcore sink: {mod_payload_path.name}")

    # PHASE 2: Domain Boundary Ingestion
    domains: List[Path] = [d for d in pkg_path.iterdir() if d.is_dir()]

    performance_path = repo_root / "performance"
    if performance_path.exists() and performance_path.is_dir():
        domains.append(performance_path)

    cmd_path = repo_root / "cmd"
    if cmd_path.exists() and cmd_path.is_dir():
        domains.append(cmd_path)

    ref_base = repo_root / "references"
    if not ref_base.exists() and (repo_root / "references!").exists(): ref_base = repo_root / "references!"
    elif not ref_base.exists(): ref_base = repo_root.parent / "references"

    go_src_path, k8s_pkg_path = ref_base / "go" / "src", ref_base / "kubernetes" / "pkg"
    k8s_staging_path = ref_base / "kubernetes" / "staging" / "src" / "k8s.io"

    split_parents = {"cmd", "crypto", "runtime", "syscall"}
    compile_internal_path = go_src_path / "cmd" / "compile" / "internal"

    if go_src_path.exists() and go_src_path.is_dir():
        for d in go_src_path.iterdir():
            if d.is_dir() and not should_exclude(d):
                if d.name in split_parents:
                    for sub in d.iterdir():
                        if sub.is_dir() and not should_exclude(sub):
                            if d.name == "cmd" and sub.name == "compile":
                                domains.append(sub)
                                if compile_internal_path.exists():
                                    domains.extend([subsub for subsub in compile_internal_path.iterdir() if subsub.is_dir() and not should_exclude(subsub)])
                            else: domains.append(sub)
                else: domains.append(d)

    k8s_pkg_domains = {"registry", "scheduler", "controller", "kubelet"}
    if k8s_pkg_path.exists() and k8s_pkg_path.is_dir():
        domains.extend([d for d in k8s_pkg_path.iterdir() if d.is_dir() and d.name in k8s_pkg_domains and not should_exclude(d)])

    k8s_staging_domains = {"apiserver", "client-go"}
    if k8s_staging_path.exists() and k8s_staging_path.is_dir():
        domains.extend([d for d in k8s_staging_path.iterdir() if d.is_dir() and d.name in k8s_staging_domains and not should_exclude(d)])

    domains.sort(key=lambda d: d.name)

    # Optimal concurrency boundary factoring in IO starvation limits
    max_workers = min(32, (os.cpu_count() or 1) * 4)

    # ThreadPool Aggregation Matrix with explicit lifecycle orchestration
    with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
        futures = []
        for domain in domains:
            is_golang_ref = (domain.is_relative_to(go_src_path) if hasattr(domain, 'is_relative_to') else "go" in domain.parts and "src" in domain.parts)
            is_k8s_ref = (domain.is_relative_to(k8s_pkg_path) or domain.is_relative_to(k8s_staging_path) if hasattr(domain, 'is_relative_to') else "kubernetes" in domain.parts)

            # Target identification
            if is_golang_ref:
                if domain == (go_src_path / "cmd" / "compile"):
                    files = sorted([f for f in domain.rglob("*") if f.is_file() and not should_exclude(f) and "internal" not in f.relative_to(domain).parts])
                else:
                    files = sorted([f for f in domain.rglob("*") if f.is_file() and not should_exclude(f)])
            elif is_k8s_ref:
                files = sorted([f for f in domain.rglob("*") if f.is_file() and not should_exclude(f) and is_k8s_allowed(f, k8s_pkg_path, k8s_staging_path)])
            elif domain.name == "performance":
                files = sorted([f for f in domain.rglob("*") if f.is_file() and not should_exclude(f)])
            elif domain.name == "cmd" and domain.parent == repo_root:
                files = sorted([f for f in domain.rglob("*") if f.is_file() and not should_exclude(f)])
            else:
                files = sorted([f for f in domain.rglob("*") if f.is_file() and not should_exclude(f) and f.suffix.lower() in [".go", ".md", ".txt", ".odt"]])

            if not files: continue

            # [OPTIMIZATION]: Zero-read byte heuristics strictly prevent physical disk fetching during setup
            word_counts = {f: get_heuristic_word_count(f) for f in files}

            chunks = split_domain(domain, files, domain, word_counts, is_root_call=True, max_words=500000)

            for chunk_data in chunks:
                futures.append(executor.submit(
                    process_domain_chunk, chunk_data, domain, is_golang_ref, is_k8s_ref,
                    compile_internal_path, output_sinks, split_parents, repo_root, ref_base
                ))

        # Enforce execution boundaries & error surface propagation via fail-fast iteration
        for future in concurrent.futures.as_completed(futures):
            try:
                future.result()
            except Exception as exc:
                print(f"[FATAL] Thread execution fractured. Forcing pipeline shutdown: {exc}")
                raise

if __name__ == "__main__":
    execute_payload_aggregation()