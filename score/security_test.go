package score

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/zegl/kube-score/config"
	ks "github.com/zegl/kube-score/domain"
	"github.com/zegl/kube-score/scorecard"
)

func TestPodSecurityContext(test *testing.T) {
	test.Parallel()

	b := func(b bool) *bool { return &b }
	i := func(i int64) *int64 { return &i }

	tests := []struct {
		ctx             *corev1.SecurityContext
		podCtx          *corev1.PodSecurityContext
		expectedGrade   scorecard.Grade
		expectedComment *scorecard.TestScoreComment
	}{
		// No security context set
		{
			ctx:           nil,
			expectedGrade: scorecard.GradeCritical,
			expectedComment: &scorecard.TestScoreComment{
				Path:        "foobar",
				Summary:     "Container has no configured security context",
				Description: "Set securityContext to run the container in a more secure context.",
			},
		},
		// All required variables set correctly
		{
			ctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: b(true),
				RunAsGroup:             i(23000),
				RunAsUser:              i(33000),
				RunAsNonRoot:           b(true),
				Privileged:             b(false),
			},
			expectedGrade: scorecard.GradeAllOK,
		},
		// Read only file system is explicitly false
		{
			ctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: b(false),
			},
			expectedGrade: scorecard.GradeCritical,
			expectedComment: &scorecard.TestScoreComment{
				Path:        "foobar",
				Summary:     "The pod has a container with a writable root filesystem",
				Description: "Set securityContext.readOnlyRootFilesystem to true",
			},
		},
		{
			ctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: b(false),
			},
			expectedGrade: scorecard.GradeCritical,
			expectedComment: &scorecard.TestScoreComment{
				Path:        "foobar",
				Summary:     "The pod has a container with a writable root filesystem",
				Description: "Set securityContext.readOnlyRootFilesystem to true",
			},
		},

		// Context is non-null, but has all null values
		{
			ctx:           &corev1.SecurityContext{},
			expectedGrade: scorecard.GradeCritical,
			expectedComment: &scorecard.TestScoreComment{
				Path:        "foobar",
				Summary:     "The pod has a container with a writable root filesystem",
				Description: "Set securityContext.readOnlyRootFilesystem to true",
			},
		},
		// Context is non nul, but has all null values
		{
			ctx:           &corev1.SecurityContext{},
			expectedGrade: scorecard.GradeCritical,
			expectedComment: &scorecard.TestScoreComment{
				Path:        "foobar",
				Summary:     "The container is running with a low user ID",
				Description: "A userid above 10 000 is recommended to avoid conflicts with the host. Set securityContext.runAsUser to a value > 10000",
			},
		},
		// Context is non nul, but has all null values
		{
			ctx:           &corev1.SecurityContext{},
			expectedGrade: scorecard.GradeCritical,
			expectedComment: &scorecard.TestScoreComment{
				Path:        "foobar",
				Summary:     "The container running with a low group ID",
				Description: "A groupid above 10 000 is recommended to avoid conflicts with the host. Set securityContext.runAsGroup to a value > 10000",
			},
		},
		// PodSecurityContext is set, assert that the values are inherited
		{
			ctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: b(true),
				RunAsNonRoot:           b(true),
				Privileged:             b(false),
			},
			podCtx: &corev1.PodSecurityContext{
				RunAsUser:  i(20000),
				RunAsGroup: i(20000),
			},
			expectedGrade: scorecard.GradeAllOK,
		},
		// PodSecurityContext is set, assert that the values are inherited
		// The container ctx has invalid values
		{
			ctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: b(true),
				RunAsNonRoot:           b(true),
				Privileged:             b(false),
				RunAsUser:              i(4),
				RunAsGroup:             i(5),
			},
			podCtx: &corev1.PodSecurityContext{
				RunAsUser:  i(20000),
				RunAsGroup: i(20000),
			},
			expectedGrade: scorecard.GradeCritical,
			expectedComment: &scorecard.TestScoreComment{
				Path:        "foobar",
				Summary:     "The container running with a low group ID",
				Description: "A groupid above 10 000 is recommended to avoid conflicts with the host. Set securityContext.runAsGroup to a value > 10000",
			},
		},

		// Privileged defaults to "false"
		{
			ctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: b(true),
				RunAsNonRoot:           b(true),
			},
			podCtx: &corev1.PodSecurityContext{
				RunAsUser:  i(20000),
				RunAsGroup: i(20000),
			},
			expectedGrade: scorecard.GradeAllOK,
		},

		// Privileged explicitly set to "false"
		{
			ctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: b(true),
				RunAsNonRoot:           b(true),
				Privileged:             b(false),
			},
			podCtx: &corev1.PodSecurityContext{
				RunAsUser:  i(20000),
				RunAsGroup: i(20000),
			},
			expectedGrade: scorecard.GradeAllOK,
		},

		// Privileged explicitly set to "true"
		{
			ctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: b(true),
				RunAsNonRoot:           b(true),
				Privileged:             b(true),
			},
			podCtx: &corev1.PodSecurityContext{
				RunAsUser:  i(20000),
				RunAsGroup: i(20000),
			},
			expectedGrade: scorecard.GradeCritical,
			expectedComment: &scorecard.TestScoreComment{
				Path:        "foobar",
				Summary:     "The container is privileged",
				Description: "Set securityContext.privileged to false. Privileged containers can access all devices on the host, and grants almost the same access as non-containerized processes on the host.",
			},
		},
	}

	for caseID, tc := range tests {
		test.Logf("Running caseID=%d", caseID)

		s := appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "foo",
					},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						SecurityContext: tc.podCtx,
						Containers: []corev1.Container{
							{
								Name:            "foobar",
								SecurityContext: tc.ctx,
							},
						},
					},
				},
			},
		}

		output, err := yaml.Marshal(s)
		assert.Nil(test, err, "caseID=%d", caseID)

		comments := testExpectedScoreWithConfig(
			test, config.Configuration{
				AllFiles:          []ks.NamedReader{unnamedReader{bytes.NewReader(output)}},
				KubernetesVersion: config.Semver{1, 18},
				EnabledOptionalTests: map[string]struct{}{
					"container-security-context": {},
				},
			},
			"Container Security Context",
			tc.expectedGrade,
		)

		// comments := testExpectedScoreReader(test, bytes.NewReader(output), "Container Security Context", tc.expectedGrade)

		if tc.expectedComment != nil {
			assert.Contains(test, comments, *tc.expectedComment, "caseID=%d", caseID)
		}
	}
}

func TestContainerSecurityContextPrivileged(t *testing.T) {
	t.Parallel()
	testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles: []ks.NamedReader{testFile("pod-security-context-privileged.yaml")},
		EnabledOptionalTests: map[string]struct{}{
			"container-security-context": {},
		},
	}, "Container Security Context", scorecard.GradeCritical)
}

func TestContainerSecurityContextLowUser(t *testing.T) {
	t.Parallel()
	testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles: []ks.NamedReader{testFile("pod-security-context-low-user-id.yaml")},
		EnabledOptionalTests: map[string]struct{}{
			"container-security-context": {},
		},
	}, "Container Security Context", scorecard.GradeCritical)
}

func TestContainerSecurityContextLowGroup(t *testing.T) {
	t.Parallel()
	testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles: []ks.NamedReader{testFile("pod-security-context-low-group-id.yaml")},
		EnabledOptionalTests: map[string]struct{}{
			"container-security-context": {},
		},
	}, "Container Security Context", scorecard.GradeCritical)
}

func TestPodSecurityContextInherited(t *testing.T) {
	t.Parallel()
	testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles: []ks.NamedReader{testFile("security-inherit-pod-security-context.yaml")},
		EnabledOptionalTests: map[string]struct{}{
			"container-security-context": {},
		},
	}, "Container Security Context", scorecard.GradeAllOK)
}

func TestContainerSecurityContextAllGood(t *testing.T) {
	t.Parallel()
	c := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles: []ks.NamedReader{testFile("pod-security-context-all-good.yaml")},
		EnabledOptionalTests: map[string]struct{}{
			"container-security-context": {},
		},
	}, "Container Security Context", scorecard.GradeAllOK)
	assert.Empty(t, c)
}

func TestContainerSeccompMissing(t *testing.T) {
	t.Parallel()

	structMap := make(map[string]struct{})
	structMap["container-seccomp-profile"] = struct{}{}

	testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-seccomp-no-annotation.yaml")},
		EnabledOptionalTests: structMap,
	}, "Container Seccomp Profile", scorecard.GradeWarning)
}

func TestContainerSeccompAllGood(t *testing.T) {
	t.Parallel()

	structMap := make(map[string]struct{})
	structMap["container-seccomp-profile"] = struct{}{}

	testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-seccomp-annotated.yaml")},
		EnabledOptionalTests: structMap,
	}, "Container Seccomp Profile", scorecard.GradeAllOK)
}

func TestContainerSecurityContextUserGroupIDAllGood(t *testing.T) {
	t.Parallel()
	structMap := make(map[string]struct{})
	structMap["container-security-context-user-group-id"] = struct{}{}
	c := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-all-good.yaml")},
		EnabledOptionalTests: structMap,
	}, "Container Security Context User Group ID", scorecard.GradeAllOK)
	assert.Empty(t, c)
}

func TestContainerSecurityContextUserGroupIDLowGroup(t *testing.T) {
	t.Parallel()
	optionalChecks := make(map[string]struct{})
	optionalChecks["container-security-context-user-group-id"] = struct{}{}
	comments := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-low-group-id.yaml")},
		EnabledOptionalTests: optionalChecks,
	}, "Container Security Context User Group ID", scorecard.GradeCritical)
	assert.Contains(t, comments, scorecard.TestScoreComment{
		Path:        "foobar",
		Summary:     "The container running with a low group ID",
		Description: "A groupid above 10 000 is recommended to avoid conflicts with the host. Set securityContext.runAsGroup to a value > 10000",
	})
}

func TestContainerSecurityContextUserGroupIDLowUser(t *testing.T) {
	t.Parallel()
	optionalChecks := make(map[string]struct{})
	optionalChecks["container-security-context-user-group-id"] = struct{}{}
	comments := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-low-user-id.yaml")},
		EnabledOptionalTests: optionalChecks,
	}, "Container Security Context User Group ID", scorecard.GradeCritical)
	assert.Contains(t, comments, scorecard.TestScoreComment{
		Path:        "foobar",
		Summary:     "The container is running with a low user ID",
		Description: "A userid above 10 000 is recommended to avoid conflicts with the host. Set securityContext.runAsUser to a value > 10000",
	})
}

func TestContainerSecurityContextUserGroupIDNoSecurityContext(t *testing.T) {
	t.Parallel()
	optionalChecks := make(map[string]struct{})
	optionalChecks["container-security-context-user-group-id"] = struct{}{}
	comments := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-nosecuritycontext.yaml")},
		EnabledOptionalTests: optionalChecks,
	}, "Container Security Context User Group ID", scorecard.GradeCritical)
	assert.Contains(t, comments, scorecard.TestScoreComment{
		Path:        "foobar",
		Summary:     "Container has no configured security context",
		Description: "Set securityContext to run the container in a more secure context.",
	})
}

func TestContainerSecurityContextPrivilegedAllGood(t *testing.T) {
	t.Parallel()
	structMap := make(map[string]struct{})
	structMap["container-security-context-privileged"] = struct{}{}
	c := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-all-good.yaml")},
		EnabledOptionalTests: structMap,
	}, "Container Security Context Privileged", scorecard.GradeAllOK)
	assert.Empty(t, c)
}

func TestContainerSecurityContextPrivilegedPrivileged(t *testing.T) {
	t.Parallel()
	optionalChecks := make(map[string]struct{})
	optionalChecks["container-security-context-privileged"] = struct{}{}
	comments := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-privileged.yaml")},
		EnabledOptionalTests: optionalChecks,
	}, "Container Security Context Privileged", scorecard.GradeCritical)
	assert.Contains(t, comments, scorecard.TestScoreComment{
		Path:        "foobar",
		Summary:     "The container is privileged",
		Description: "Set securityContext.privileged to false. Privileged containers can access all devices on the host, and grants almost the same access as non-containerized processes on the host.",
	})
}

func TestContainerSecurityContextReadOnlyRootFilesystemAllGood(t *testing.T) {
	t.Parallel()
	structMap := make(map[string]struct{})
	structMap["container-security-context-readonlyrootfilesystem"] = struct{}{}
	c := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-all-good.yaml")},
		EnabledOptionalTests: structMap,
	}, "Container Security Context ReadOnlyRootFilesystem", scorecard.GradeAllOK)
	assert.Empty(t, c)
}

func TestContainerSecurityContextReadOnlyRootFilesystemWriteable(t *testing.T) {
	t.Parallel()
	optionalChecks := make(map[string]struct{})
	optionalChecks["container-security-context-readonlyrootfilesystem"] = struct{}{}
	comments := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-writeablerootfilesystem.yaml")},
		EnabledOptionalTests: optionalChecks,
	}, "Container Security Context ReadOnlyRootFilesystem", scorecard.GradeCritical)
	assert.Contains(t, comments, scorecard.TestScoreComment{
		Path:        "foobar",
		Summary:     "The pod has a container with a writable root filesystem",
		Description: "Set securityContext.readOnlyRootFilesystem to true",
	})
}

func TestContainerSecurityContextReadOnlyRootFilesystemNoSecurityContext(t *testing.T) {
	t.Parallel()
	optionalChecks := make(map[string]struct{})
	optionalChecks["container-security-context-readonlyrootfilesystem"] = struct{}{}
	comments := testExpectedScoreWithConfig(t, config.Configuration{
		AllFiles:             []ks.NamedReader{testFile("pod-security-context-nosecuritycontext.yaml")},
		EnabledOptionalTests: optionalChecks,
	}, "Container Security Context ReadOnlyRootFilesystem", scorecard.GradeCritical)
	assert.Contains(t, comments, scorecard.TestScoreComment{
		Path:        "foobar",
		Summary:     "Container has no configured security context",
		Description: "Set securityContext to run the container in a more secure context.",
	})
}
