import sys
import zipfile
import re
import xml.etree.ElementTree as ET
from pathlib import Path

# Noise pattern matching directory components of (vendor, testdata, _gen, test, misc, .git)
# or filenames ending with _gen.go or _test.go
noise_rx = re.compile(r'(?:[\\/](?:vendor|testdata|_gen|test|misc|\.git)(?:[\\/]|$))|_(?:gen|test)\.go$', re.IGNORECASE)

def should_exclude(path: Path) -> bool:
    """Returns True if the path matches the noise pattern."""
    posix_path = path.as_posix()
    return bool(noise_rx.search(posix_path))

def generate_topology(directory: Path, prefix: str = "", exclude_fn=None) -> str:
    """
    Recursively builds a deterministic directory tree visualization.
    Executes an O(N) traversal. Output is aggregated in RAM before return.
    """
    tree_buffer = []
    # Filter paths if exclude_fn is provided
    paths = [p for p in directory.iterdir() if exclude_fn is None or not exclude_fn(p)]
    # Enforce deterministic ordering: directories first, then files, alphabetically sorted
    paths = sorted(paths, key=lambda p: (not p.is_dir(), p.name))
    pointers = [('├── ', '│   ')] * (len(paths) - 1) + [('└── ', '    ')] if paths else []
    
    for pointer, path in zip(pointers, paths):
        tree_buffer.append(f"{prefix}{pointer[0]}{path.name}\n")
        if path.is_dir():
            tree_buffer.append(generate_topology(path, prefix + pointer[1], exclude_fn))
            
    return "".join(tree_buffer)

def convert_odt_to_markdown(odt_path: Path) -> str:
    """
    Converts an OpenDocument Text (.odt) file into basic Markdown format.
    Extracts headings based on style parent relationships, lists, tables, and paragraphs.
    """
    NS = {
        'office': 'urn:oasis:names:tc:opendocument:xmlns:office:1.0',
        'text': 'urn:oasis:names:tc:opendocument:xmlns:text:1.0',
        'table': 'urn:oasis:names:tc:opendocument:xmlns:table:1.0',
        'style': 'urn:oasis:names:tc:opendocument:xmlns:style:1.0',
    }

    def clean_tag(tag):
        return tag.split('}')[-1]

    def extract_text(element):
        parts = []
        if element.text:
            parts.append(element.text)
        for child in element:
            parts.append(extract_text(child))
            if child.tail:
                parts.append(child.tail)
        return "".join(parts)

    try:
        with zipfile.ZipFile(odt_path) as z:
            content_xml = z.read("content.xml")
            root = ET.fromstring(content_xml)
            
            # Map automatic styles to their parent styles to identify headings
            style_parent_map = {}
            auto_styles = root.find('.//office:automatic-styles', NS)
            if auto_styles is not None:
                for style in auto_styles:
                    name = style.attrib.get(f"{{{NS['style']}}}name", "")
                    parent = style.attrib.get(f"{{{NS['style']}}}parent-style-name", "")
                    if name and parent:
                        style_parent_map[name] = parent
            
            def get_heading_level(style_name):
                if not style_name:
                    return 0
                parent = style_parent_map.get(style_name, style_name)
                if parent == 'Heading_20_1' or parent.startswith('Heading 1') or parent.endswith('1'):
                    return 1
                elif parent == 'Heading_20_2' or parent.startswith('Heading 2') or parent.endswith('2'):
                    return 2
                elif parent == 'Heading_20_3' or parent.startswith('Heading 3') or parent.endswith('3'):
                    return 3
                elif parent == 'Heading_20_4' or parent.startswith('Heading 4') or parent.endswith('4'):
                    return 4
                return 0

            body = root.find('.//office:body', NS)
            if body is None:
                return ""
            text_body = body.find('.//office:text', NS)
            if text_body is None:
                return ""

            md_lines = []
            
            def process_element(element):
                tag = clean_tag(element.tag)
                if tag == 'p':
                    style_name = element.attrib.get(f"{{{NS['text']}}}style-name", "")
                    level = get_heading_level(style_name)
                    txt = extract_text(element).strip()
                    if txt:
                        if level > 0:
                            md_lines.append(f"{'#' * level} {txt}\n")
                        else:
                            md_lines.append(f"{txt}\n")
                elif tag == 'list':
                    for item in element.findall('.//text:list-item', NS):
                        for p in item.findall('.//text:p', NS):
                            txt = extract_text(p).strip()
                            if txt:
                                md_lines.append(f"- {txt}")
                    md_lines.append("")
                elif tag == 'table':
                    rows = element.findall('.//table:table-row', NS)
                    if not rows:
                        return
                    md_rows = []
                    for r_idx, row in enumerate(rows):
                        cells = row.findall('.//table:table-cell', NS)
                        cell_texts = []
                        for cell in cells:
                            p_texts = [extract_text(p).strip() for p in cell.findall('.//text:p', NS)]
                            cell_texts.append(" ".join(filter(None, p_texts)))
                        md_rows.append(f"| {' | '.join(cell_texts)} |")
                        if r_idx == 0:
                            seps = ['---'] * len(cell_texts)
                            md_rows.append(f"| {' | '.join(seps)} |")
                    md_lines.extend(md_rows)
                    md_lines.append("")
            
            for child in text_body:
                process_element(child)
                
            return "\n".join(md_lines)
    except Exception as e:
        return f"// [I/O FAULT]: Failed to convert ODT - {e}\n"

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
    root_target_files = ["ARCHITECTURE.md", "AGENTS.md", "softmax.md"]
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
    # PHASE 2: Domain Boundary Ingestion (./pkg/* & ./performance & references/go/src/*)
    # =================================================================
    domains = [d for d in pkg_path.iterdir() if d.is_dir()]
    performance_path = repo_root / "performance"
    if performance_path.exists() and performance_path.is_dir():
        domains.append(performance_path)
    
    references_path = repo_root / "references" / "go" / "src"
    if not references_path.exists():
        references_path = repo_root.parent / "references" / "go" / "src"
    split_parents = {"cmd", "crypto", "runtime", "syscall"}
    compile_internal_path = references_path / "cmd" / "compile" / "internal"

    if references_path.exists() and references_path.is_dir():
        for d in references_path.iterdir():
            if d.is_dir() and not should_exclude(d):
                if d.name in split_parents:
                    # Treat immediate subfolders inside d as individual domains
                    for sub in d.iterdir():
                        if sub.is_dir() and not should_exclude(sub):
                            if d.name == "cmd" and sub.name == "compile":
                                # Add cmd/compile itself as a domain
                                domains.append(sub)
                                # Also add every subfolder inside cmd/compile/internal
                                if compile_internal_path.exists() and compile_internal_path.is_dir():
                                    for subsub in compile_internal_path.iterdir():
                                        if subsub.is_dir() and not should_exclude(subsub):
                                            domains.append(subsub)
                            else:
                                domains.append(sub)
                else:
                    domains.append(d)
    
    domains.sort(key=lambda d: d.name)

    for domain in domains:
        is_golang_ref = False
        try:
            is_golang_ref = domain.is_relative_to(references_path)
        except Exception:
            is_golang_ref = "references" in domain.parts and "go" in domain.parts and "src" in domain.parts

        # Determine output payload name and display name
        if is_golang_ref:
            is_compile_internal_sub = False
            try:
                is_compile_internal_sub = domain.is_relative_to(compile_internal_path) and domain != compile_internal_path
            except Exception:
                is_compile_internal_sub = "compile" in domain.parts and "internal" in domain.parts and domain.parent.name == "internal"
            
            if is_compile_internal_sub:
                domain_display_name = f"golang_cmd_compile_internal_{domain.name}"
                payload_path = output_sink / f"golang_cmd_compile_internal_{domain.name}.md"
                arch_title = f"cmd/compile/internal/{domain.name}"
            else:
                parent_name = domain.parent.name
                if parent_name in split_parents:
                    domain_display_name = f"golang_{parent_name}_{domain.name}"
                    payload_path = output_sink / f"golang_{parent_name}_{domain.name}.md"
                    arch_title = f"{parent_name}/{domain.name}"
                else:
                    domain_display_name = f"golang_{domain.name}"
                    payload_path = output_sink / f"golang_{domain.name}.md"
                    arch_title = domain.name
        else:
            domain_display_name = domain.name
            payload_path = output_sink / f"{domain.name}_notebook_payload.md"
            arch_title = domain.name

        print(f"[-] Compiling domain boundary: {domain_display_name}")
        
        # Initialize contiguous memory buffer for this domain
        if is_golang_ref and domain == (references_path / "cmd" / "compile"):
            exclude_fn = lambda p: should_exclude(p) or p.name == "internal"
        elif is_golang_ref:
            exclude_fn = should_exclude
        else:
            exclude_fn = None
        
        domain_buffer = [
            f"# Domain Architecture: {arch_title}\n\n",
            "## Layout Topology\n```text\n",
            f"{arch_title}/\n",
            generate_topology(domain, exclude_fn=exclude_fn),
            "```\n\n",
            "## Source Stream Aggregation\n\n"
        ]
        
        # Sequential file ingestion into RAM
        if is_golang_ref:
            if domain == (references_path / "cmd" / "compile"):
                files = sorted([f for f in domain.rglob("*") if f.is_file() and not should_exclude(f) and not any(p == "internal" for p in f.relative_to(domain).parts)])
            else:
                files = sorted([f for f in domain.rglob("*") if f.is_file() and not should_exclude(f)])
        elif domain.name == "performance":
            files = sorted([f for f in domain.rglob("*") if f.is_file()])
        else:
            files = sorted(domain.rglob("*.go"))

        if not files:
            domain_buffer.append("> [WARN] 0x00 source streams detected within this layout boundary.\n")
            # Flush empty state to disk
            payload_path.write_text("".join(domain_buffer), encoding="utf-8")
            continue
        
        for file_path in files:
            try:
                relative_path = file_path.relative_to(repo_root).as_posix()
            except ValueError:
                relative_path = (Path("references") / file_path.relative_to(references_path.parent.parent)).as_posix()
            
            if file_path.suffix.lower() == ".odt":
                display_path = relative_path.replace(".odt", ".md")
                domain_buffer.append(f"// === FILE: {display_path} ===\n```markdown\n")
                try:
                    domain_buffer.append(convert_odt_to_markdown(file_path))
                except Exception as e:
                    domain_buffer.append(f"// [I/O FAULT]: Failed to map memory boundary - {e}\n")
                domain_buffer.append("\n```\n\n")
            else:
                lang = "go" if file_path.suffix.lower() == ".go" else "markdown" if file_path.suffix.lower() == ".md" else "text"
                domain_buffer.append(f"// === FILE: {relative_path} ===\n```{lang}\n")
                try:
                    domain_buffer.append(file_path.read_text(encoding="utf-8"))
                except Exception as e:
                    domain_buffer.append(f"// [I/O FAULT]: Failed to map memory boundary - {e}\n")
                domain_buffer.append("\n```\n\n")
            
        # Atomic flush from RAM to persistent disk
        payload_path.write_text("".join(domain_buffer), encoding="utf-8")
        print(f"[+] Domain execution complete. Payload routed to sink: {payload_path.name}")

if __name__ == "__main__":
    execute_payload_aggregation()