package adversarial

// BuiltinSpecs returns the shipped deterministic taxonomy.
func BuiltinSpecs() []Spec {
	specs := []Spec{}
	specs = append(specs, builtinDeterministicSpecsA()...)
	specs = append(specs, builtinDeterministicSpecsB()...)
	specs = append(specs, builtinLLMContradictionSpec())
	specs = append(specs, builtinExplorationSpecs022To038()...)
	specs = append(specs, builtinExplorationSpecs039To047()...)
	specs = append(specs, builtinExplorationSpecs048To056()...)
	specs = append(specs, builtinExplorationSpecs057To069()...)
	specs = append(specs, builtinExplorationSpecs070To080()...)
	specs = append(specs, builtinExplorationSpecs081To093()...)
	specs = append(specs, builtinExplorationSpecs094To100()...)
	specs = append(specs, builtinExplorationSpecs101To105()...)
	specs = append(specs, builtinExplorationSpecs106To110()...)
	specs = append(specs, builtinExplorationSpecs111To115()...)
	specs = append(specs, builtinExplorationSpecs116To120()...)
	specs = append(specs, builtinExplorationSpecs121To125()...)
	specs = append(specs, builtinExplorationSpecs126To130()...)
	specs = append(specs, builtinExplorationSpecs131To135()...)
	specs = append(specs, builtinExplorationSpecs136To140()...)
	return specs
}
