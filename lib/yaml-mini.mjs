function stripComment(line) {
  let inSingle = false;
  let inDouble = false;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (c === "'" && !inDouble) inSingle = !inSingle;
    else if (c === '"' && !inSingle) inDouble = !inDouble;
    else if (c === "#" && !inSingle && !inDouble) return line.slice(0, i);
  }
  return line;
}

function unquote(raw) {
  const t = raw.trim();
  if (t === "") return "";
  if ((t.startsWith('"') && t.endsWith('"')) || (t.startsWith("'") && t.endsWith("'"))) {
    return t.slice(1, -1);
  }
  if (t === "true") return true;
  if (t === "false") return false;
  if (t === "null" || t === "~") return null;
  if (/^-?\d+$/.test(t)) return Number(t);
  if (/^-?\d+\.\d+$/.test(t)) return Number(t);
  return t;
}

function indent(line) {
  let n = 0;
  while (n < line.length && line[n] === " ") n++;
  return n;
}

function tokenize(src) {
  const out = [];
  for (const rawLine of src.split(/\r?\n/)) {
    const line = stripComment(rawLine).replace(/\s+$/, "");
    if (line.trim() === "") continue;
    out.push({ indent: indent(line), text: line.trim(), raw: line });
  }
  return out;
}

function parseBlock(lines, start, baseIndent) {
  // Decide: list of items, or map of keys.
  if (start >= lines.length) return [{}, start];
  const first = lines[start];
  if (first.indent < baseIndent) return [{}, start];
  if (first.text.startsWith("- ")) {
    return parseList(lines, start, baseIndent);
  }
  return parseMap(lines, start, baseIndent);
}

function parseMap(lines, start, baseIndent) {
  const obj = {};
  let i = start;
  while (i < lines.length) {
    const ln = lines[i];
    if (ln.indent < baseIndent) break;
    if (ln.indent > baseIndent) {
      throw new Error(`yaml-mini: unexpected indent at "${ln.raw}"`);
    }
    const m = ln.text.match(/^([A-Za-z0-9_.-]+):\s*(.*)$/);
    if (!m) throw new Error(`yaml-mini: expected key: at "${ln.raw}"`);
    const key = m[1];
    const rest = m[2];
    if (rest === "" || rest === "|") {
      // nested block
      i++;
      if (i < lines.length && lines[i].indent > baseIndent) {
        const childIndent = lines[i].indent;
        const [val, next] = parseBlock(lines, i, childIndent);
        obj[key] = val;
        i = next;
      } else {
        obj[key] = null;
      }
    } else {
      obj[key] = unquote(rest);
      i++;
    }
  }
  return [obj, i];
}

function parseList(lines, start, baseIndent) {
  const arr = [];
  let i = start;
  while (i < lines.length) {
    const ln = lines[i];
    if (ln.indent < baseIndent) break;
    if (!ln.text.startsWith("- ") && ln.text !== "-") {
      break;
    }
    const after = ln.text === "-" ? "" : ln.text.slice(2);
    if (after === "") {
      i++;
      if (i < lines.length && lines[i].indent > baseIndent) {
        const childIndent = lines[i].indent;
        const [val, next] = parseBlock(lines, i, childIndent);
        arr.push(val);
        i = next;
      } else {
        arr.push(null);
      }
    } else if (/^[A-Za-z0-9_.-]+:\s*/.test(after)) {
      // inline map item: "- key: val"
      const inlineIndent = baseIndent + 2;
      const synthetic = [{ indent: inlineIndent, text: after, raw: " ".repeat(inlineIndent) + after }];
      let j = i + 1;
      while (j < lines.length && lines[j].indent >= inlineIndent) {
        synthetic.push(lines[j]);
        j++;
      }
      const [val, nextLocal] = parseMap(synthetic, 0, inlineIndent);
      arr.push(val);
      i = i + (nextLocal - 1) + 1;
    } else {
      arr.push(unquote(after));
      i++;
    }
  }
  return [arr, i];
}

export function parseYaml(src) {
  const lines = tokenize(src);
  if (lines.length === 0) return {};
  const baseIndent = lines[0].indent;
  const [val] = parseBlock(lines, 0, baseIndent);
  return val;
}
