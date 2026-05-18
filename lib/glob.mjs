function escapeRegex(s) {
  return s.replace(/[.+^$(){}|\\]/g, "\\$&");
}

export function globToRegex(glob) {
  const parts = [];
  let i = 0;
  while (i < glob.length) {
    const c = glob[i];
    if (c === "*" && glob[i + 1] === "*") {
      if (glob[i + 2] === "/") {
        parts.push("(?:.*/)?");
        i += 3;
      } else {
        parts.push(".*");
        i += 2;
      }
    } else if (c === "*") {
      parts.push("[^/]*");
      i += 1;
    } else if (c === "?") {
      parts.push("[^/]");
      i += 1;
    } else {
      parts.push(escapeRegex(c));
      i += 1;
    }
  }
  return new RegExp("^" + parts.join("") + "$");
}

export function matches(glob, path) {
  return globToRegex(glob).test(path);
}

export function anyMatches(globs, paths) {
  return globs.some((g) => {
    const re = globToRegex(g);
    return paths.some((p) => re.test(p));
  });
}
