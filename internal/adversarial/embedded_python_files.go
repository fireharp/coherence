package adversarial

func embeddedPythonFiles() map[string]string {
	return map[string]string{
		"pyapp/__init__.py":        "",
		"pyapp/plugin.py":          "VALUE = 1\n",
		"pyapp/abs_plugin.py":      "value = 1\n",
		"pyapp/abs_consumer.py":    "from pyapp.abs_plugin import value\n\nresult = value\n",
		"pyapp/imported_module.py": "value = 1\n",
		"pyapp/import_consumer.py": "import pyapp.imported_module\n\nresult = pyapp.imported_module.value\n",
		"pyapp/dot_import_dep.py":  "value = 1\n",
		"pyapp/dot_import_consumer.py": `from . import dot_import_dep

result = dot_import_dep.value
`,
		"pyapp/calc.py":    "def calc_value():\n    return 1\n",
		"pyapp/cycle_a.py": "from .cycle_b import value_b\n\nvalue_a = value_b + 1\n",
		"pyapp/cycle_b.py": "value_b = 1\n",
		"tests/test_calc.py": `from pyapp.calc import calc_value

def test_calc_value():
    assert calc_value() == 1
`,
		"pyapp/loader.py": `import importlib

PLUGIN_MODULE = ".plugin"

def load_plugin():
    return importlib.import_module(PLUGIN_MODULE, __package__)
`,
	}
}
