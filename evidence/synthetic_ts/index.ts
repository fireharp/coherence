// Synthetic TS corpus with ground truth markers for dead_code precision.

// GT: live (entrypoint)
export function main() {
  const s = new Service();
  s.activeMethod();
  freeHelper();
}

// GT: live (constructed)
export class Service {
  // GT: live (called from main)
  activeMethod() {
    return this.internalHelper();
  }

  // GT: live (called by activeMethod)
  private internalHelper() {
    return 1;
  }

  // GT: dead-code (private, no callers)
  private orphanMethod() {
    return 2;
  }
}

// GT: dead-code (class never instantiated, no callers)
export class UnusedService {
  // GT: dead-code (parent never instantiated)
  doWork() {
    return 3;
  }
  // GT: dead-code
  private innerHelper() {
    return 4;
  }
}

// GT: live (called from main)
function freeHelper() {
  return innerFree();
}

// GT: live (called transitively)
function innerFree() {
  return 5;
}

// GT: dead-code (free function, no callers)
function orphanFreeFunc() {
  return 6;
}

// GT: live as library API (exported, no in-tree callers)
export function libraryAPI() {
  return innerFree();
}
