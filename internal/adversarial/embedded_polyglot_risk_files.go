package adversarial

func embeddedPolyglotRiskFiles() map[string]string {
	return map[string]string{
		"crates/risk/src/lib.rs":           "pub mod policy_config;\n\npub fn risk_limit() -> i32 { 7 }\n",
		"crates/risk/src/policy_config.rs": "pub const POLICY_CONFIG: &str = \"default\";\n",
		"tests/risk_limit_test.rs": `use risk::risk_limit;

#[test]
fn checks_limit() {
    assert_eq!(risk_limit(), 7);
}
`,
		"java/src/main/java/com/example/RiskPolicy.java": `package com.example;

public final class RiskPolicy {
    public static int limit() {
        return 7;
    }
}
`,
		"java/src/test/java/com/example/RiskPolicyTest.java": `package com.example;

import static org.junit.jupiter.api.Assertions.assertEquals;
import org.junit.jupiter.api.Test;

final class RiskPolicyTest {
    @Test
    void checksLimit() {
        assertEquals(7, RiskPolicy.limit());
    }
}
`,
		"csharp/RiskPolicy.cs": `namespace Example;

public static class RiskPolicy
{
    public static int Limit() => 7;
}
`,
		"csharp/RiskPolicyTests.cs": `using Xunit;

namespace Example.Tests;

public sealed class RiskPolicyTests
{
    [Fact]
    public void ChecksLimit()
    {
        Assert.Equal(7, RiskPolicy.Limit());
    }
}
`,
	}
}
