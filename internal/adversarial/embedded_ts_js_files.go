package adversarial

func embeddedTSJSFiles() map[string]string {
	return map[string]string{
		"src/util.ts":             "export const util = 1;\n",
		"src/usesUtil.ts":         "import { util } from './util';\nexport const value = util;\n",
		"src/reexported.ts":       "export const reexported = 1;\n",
		"src/barrel.ts":           "export { reexported } from './reexported';\nexport * from './reexported';\n",
		"src/lazy.ts":             "export const lazy = 1;\n",
		"src/loadLazy.ts":         "export async function loadLazy() {\n  return import('./lazy');\n}\n",
		"src/cjsDep.ts":           "export const requiredValue = 1;\n",
		"src/requireConsumer.ts":  "const dep = require(\"./cjsDep\");\nexport const requiredValue = dep.requiredValue;\n",
		"src/importEqualsDep.ts":  "export const importEqualsValue = 1;\n",
		"src/importEqualsUser.ts": "import dep = require(\"./importEqualsDep\");\nexport const importEqualsValue = dep.importEqualsValue;\n",
		"src/multilineDep.ts":     "export const multilineValue = 1;\n",
		"src/multilineImport.ts": `import {
  multilineValue,
} from "./multilineDep";

export const multilineImportValue = multilineValue;
`,
		"src/types.d.ts":   "export interface WidgetConfig { enabled: boolean }\n",
		"src/usesTypes.ts": "/// <reference path=\"./types.d.ts\" />\nexport const enabled = true;\n",
		"src/aliased.ts":   "export const aliased = 1;\n",
		"src/usesAlias.ts": "import { aliased } from '@/aliased';\nexport const aliasValue = aliased;\n",
		"src/jsDep.js":     "export const jsDep = 1;\n",
		"src/jsUsesDep.js": "import { jsDep } from './jsDep.js';\nexport const jsValue = jsDep;\n",
		"src/widget.ts":    "export function widgetValue() {\n  return 1;\n}\n",
		"src/__tests__/widget.test.ts": `import { widgetValue } from "../widget";

test("widget value", () => {
  expect(widgetValue()).toBe(1);
});
`,
		"src/esmWidget.mjs": "export function esmWidgetValue() {\n  return 1;\n}\n",
		"src/esmWidget.test.mjs": `import { esmWidgetValue } from "./esmWidget.mjs";

test("esm widget value", () => {
  expect(esmWidgetValue()).toBe(1);
});
`,
		"src/a/index.ts": "import { b } from '../b';\nexport const a = b;\n",
		"src/b/index.ts": "import { c } from '../c';\nexport const b = c;\n",
		"src/c/index.ts": "export const c = 1;\n",
		"src/api.ts": `const app = express();
app.get("/api/orders", getOrders);
function getOrders(req, res) { res.send("ok"); }
`,
		"src/api.test.ts": `import "./api";
test("orders endpoint", () => {
  expect(true).toBe(true);
});
`,
	}
}
