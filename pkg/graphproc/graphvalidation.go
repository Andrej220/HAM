package graphproc

import (
	"regexp"
	"strings"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)


var validate = validator.New()

func init() {
	validate = validator.New()
	
	// Register custom validations
	_ = validate.RegisterValidation("validNodeType", validateNodeType)
	_ = validate.RegisterValidation("validNodeID", validateNodeID)
	_ = validate.RegisterValidation("acyclic", validateAcyclic)
}

func validateNodeType(fl validator.FieldLevel) bool {
	validTypes := map[string]bool{
		"exec":       true,
		"condition":  true,
		"aggregate":  true,
		"transform":  true,
		"":           true, // Allow empty type
	}
	return validTypes[fl.Field().String()]
}

func validateNodeID(fl validator.FieldLevel) bool {
	id := fl.Field().String()
	if id == "" {
		return false
	}
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, id)
	return match
}

func validateAcyclic(fl validator.FieldLevel) bool {
	root := fl.Field().Interface().(*Node)
	return !isCyclic(root, make(map[string]bool))
}

func isCyclic(node *Node, visited map[string]bool) bool {
	if node == nil {
		return false
	}
	
	if visited[node.ID] {
		return true
	}
	
	visited[node.ID] = true
	defer delete(visited, node.ID)
	
	for _, child := range node.Children {
		if isCyclic(child, visited) {
			return true
		}
	}
	return false
}

func ValidateConfig(cfg *Config) error {
	return validate.Struct(cfg)
}

func ValidateGraph(graph *Graph) error {
	if err := ValidateConfig(graph.Config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	if err := validateNodeTree(graph.Root); err != nil {
		return fmt.Errorf("node validation failed: %w", err)
	}

	if isCyclic(graph.Root, make(map[string]bool)) {
		return fmt.Errorf("graph contains cycles")
	}

	if graph.UUID != uuid.Nil {
		if _, err := uuid.Parse(graph.UUID.String()); err != nil {
			return fmt.Errorf("invalid UUID: %w", err)
		}
	}

	return nil
}

func validateNodeTree(node *Node) error {
	if node == nil {
		return nil
	}

	if err := ValidateNode(node); err != nil {
		return err
	}

	for _, child := range node.Children {
		if err := validateNodeTree(child); err != nil {
			return err
		}
	}

	return nil
}

func ValidateNode(node *Node) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}

	if err := validate.Struct(node); err != nil {
		return err
	}

	if node.Type == "exec" && strings.TrimSpace(node.Script) == "" {
		return fmt.Errorf("script is required for nodes of type 'exec'")
	}

	if len(node.Result) > 0 {
		for i, result := range node.Result {
			if strings.TrimSpace(result) == "" {
				return fmt.Errorf("result at index %d cannot be empty", i)
			}
		}
	}

	if len(node.Stderr) > 0 {
		for i, errMsg := range node.Stderr {
			if strings.TrimSpace(errMsg) == "" {
				return fmt.Errorf("error message at index %d cannot be empty", i)
			}
		}
	}

	if node.PostProcess != "" {
		if !strings.HasPrefix(node.PostProcess, "@") {
			return fmt.Errorf("post_process must start with '@'")
		}
	}

	return nil
}