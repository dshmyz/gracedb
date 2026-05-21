package graph

// BFS performs a breadth-first traversal from a start node.
func (g *GraphStore) BFS(startNodeID string, opts NeighborOptions) (*TraversalResult, error) {
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 3
	}

	visited := make(map[string]bool)
	visited[startNodeID] = true

	var resultNodes []*GraphNode
	var resultEdges []*GraphEdge
	queue := []string{startNodeID}
	depth := 0

	for len(queue) > 0 && depth < opts.MaxDepth {
		nextLevel := make([]string, 0)
		for _, nodeID := range queue {
			neighbors, edges, err := g.GetNeighbors(nodeID, opts)
			if err != nil {
				continue
			}

			for i, n := range neighbors {
				if !visited[n.ID] {
					visited[n.ID] = true
					nextLevel = append(nextLevel, n.ID)
					resultNodes = append(resultNodes, n)
					if i < len(edges) {
						resultEdges = append(resultEdges, edges[i])
					}
				}
			}

			// Also add edges for already-visited nodes.
			for _, e := range edges {
				if visited[e.ToNodeID] || visited[e.FromNodeID] {
					resultEdges = append(resultEdges, e)
				}
			}
		}
		queue = nextLevel
		depth++
	}

	return &TraversalResult{
		Nodes: deduplicateNodes(resultNodes),
		Edges: resultEdges,
	}, nil
}

// DFS performs a depth-first traversal from a start node.
func (g *GraphStore) DFS(startNodeID string, opts NeighborOptions) (*TraversalResult, error) {
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 3
	}

	visited := make(map[string]bool)
	var resultNodes []*GraphNode
	var resultEdges []*GraphEdge

	var dfs func(nodeID string, depth int)
	dfs = func(nodeID string, depth int) {
		if visited[nodeID] || depth >= opts.MaxDepth {
			return
		}
		visited[nodeID] = true

		neighbors, edges, err := g.GetNeighbors(nodeID, opts)
		if err != nil {
			return
		}

		for i, n := range neighbors {
			if !visited[n.ID] {
				resultNodes = append(resultNodes, n)
				if i < len(edges) {
					resultEdges = append(resultEdges, edges[i])
				}
				dfs(n.ID, depth+1)
			}
		}
	}

	// Add start node.
	start, err := g.GetNode(startNodeID)
	if err == nil {
		resultNodes = append(resultNodes, start)
	}

	dfs(startNodeID, 0)

	return &TraversalResult{
		Nodes: deduplicateNodes(resultNodes),
		Edges: resultEdges,
	}, nil
}

// ShortestPath finds the shortest path between two nodes using BFS.
func (g *GraphStore) ShortestPath(fromID, toID string) (*PathResult, error) {
	if fromID == toID {
		return &PathResult{
			FromNodeID: fromID,
			ToNodeID:   toID,
			NodeIDs:    []string{fromID},
		}, nil
	}

	visited := make(map[string]bool)
	parent := make(map[string]string)   // child -> parent
	edgeTo := make(map[string]string)    // child -> edge ID from parent
	queue := []string{fromID}
	visited[fromID] = true

	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		if nodeID == toID {
			// Reconstruct path.
			var path []string
			var edgePath []string
			current := toID
			for current != fromID {
				path = append([]string{current}, path...)
				if eID, ok := edgeTo[current]; ok {
					edgePath = append([]string{eID}, edgePath...)
				}
				current = parent[current]
			}
			path = append([]string{fromID}, path...)

			return &PathResult{
				FromNodeID: fromID,
				ToNodeID:   toID,
				NodeIDs:    path,
				EdgeIDs:    edgePath,
				Length:     len(path) - 1,
			}, nil
		}

		neighbors, _, err := g.GetNeighbors(nodeID, NeighborOptions{Direction: "out"})
		if err != nil {
			continue
		}

		for _, n := range neighbors {
			if !visited[n.ID] {
				visited[n.ID] = true
				parent[n.ID] = nodeID
				// Find the edge ID from nodeID to n.ID.
				for _, edge := range getEdgesFrom(g, nodeID) {
					if edge.ToNodeID == n.ID {
						edgeTo[n.ID] = edge.ID
						break
					}
				}
				queue = append(queue, n.ID)
			}
		}
	}

	return nil, nil // No path found
}

func getEdgesFrom(g *GraphStore, nodeID string) []*GraphEdge {
	neighbors, edges, _ := g.GetNeighbors(nodeID, NeighborOptions{Direction: "out"})
	_ = neighbors
	return edges
}

func deduplicateNodes(nodes []*GraphNode) []*GraphNode {
	seen := make(map[string]bool)
	var result []*GraphNode
	for _, n := range nodes {
		if !seen[n.ID] {
			seen[n.ID] = true
			result = append(result, n)
		}
	}
	return result
}
