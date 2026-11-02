package quint

import "fmt"

// WalkExpr visits expressions in preorder. Returning false prunes the current
// subtree, while an error stops traversal. Let definitions precede their body.
// The callback receives the original nodes and must not mutate the tree.
func WalkExpr(root *Expr, visit func(*Expr) (bool, error)) error {
	if root == nil {
		return nil
	}
	descend, err := visit(root)
	if err != nil || !descend {
		return err
	}
	switch root.Kind {
	case "name", "bool", "int", "str":
		return nil
	case "app":
		for i := range root.Args {
			if err := WalkExpr(&root.Args[i], visit); err != nil {
				return err
			}
		}
	case "lambda":
		return WalkExpr(root.Body, visit)
	case "let":
		if root.Opdef == nil {
			return fmt.Errorf("let expression missing definition")
		}
		if err := WalkExpr(root.Opdef.Expr, visit); err != nil {
			return err
		}
		return WalkExpr(root.Body, visit)
	default:
		return fmt.Errorf("unknown expression kind %q", root.Kind)
	}
	return nil
}
func FindExpr(root *Expr, id ID) *Expr {
	var found *Expr
	_ = WalkExpr(root, func(e *Expr) (bool, error) {
		if found != nil {
			return false, nil
		}
		if e.ID == id {
			found = e
			return false, nil
		}
		return true, nil
	})
	return found
}

// WalkType traverses type children in source order, including row field types.
func WalkType(root *Type, visit func(*Type) (bool, error)) error {
	if root == nil {
		return nil
	}
	descend, err := visit(root)
	if err != nil || !descend {
		return err
	}
	switch root.Kind {
	case "bool", "int", "str", "const", "var":
		return nil
	case "set", "list":
		return WalkType(root.Elem, visit)
	case "fun":
		if err := WalkType(root.Arg, visit); err != nil {
			return err
		}
		return WalkType(root.Res, visit)
	case "oper", "app":
		if root.Kind == "app" {
			if err := WalkType(root.Ctor, visit); err != nil {
				return err
			}
		}
		for i := range root.Args {
			if err := WalkType(&root.Args[i], visit); err != nil {
				return err
			}
		}
		if root.Kind == "oper" {
			return WalkType(root.Res, visit)
		}
	case "tup", "rec", "sum":
		for row := root.Fields; row != nil; row = row.Other {
			for i := range row.Fields {
				if err := WalkType(&row.Fields[i].Type, visit); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unknown type kind %q", root.Kind)
	}
	return nil
}
