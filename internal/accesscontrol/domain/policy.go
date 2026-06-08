package domain

func HasResourceScopes(resources []ResourceAccess, requiredResource string, requiredScopes []string) bool {
	for _, resource := range resources {
		if resource.ResourceName != requiredResource {
			continue
		}
		if len(requiredScopes) == 0 {
			return true
		}

		availableScopes := make(map[string]struct{}, len(resource.Scopes))
		for _, scope := range resource.Scopes {
			availableScopes[scope] = struct{}{}
		}
		for _, requiredScope := range requiredScopes {
			if _, ok := availableScopes[requiredScope]; !ok {
				return false
			}
		}
		return true
	}
	return false
}
