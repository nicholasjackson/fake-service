const startX = 100;
const startY = 100;
const incrX = 300;
const incrY = 300;

function processNode(node, parent, xStart, yStart) {
  var nodes = [];
  var links = [];

  var data = {
    id: node.name,
    properties: {
      name: node.name,
      duration: node.duration,
      type: node.type
    },
    position: {
      x: xStart,
      y: yStart
    },
    ports: {}
  }

  nodes.push(data);

  // if there is a parent node add the input port
  if (parent != null) {
    data.ports["input0"] = { id: "input0", type: "input" };
  }

  // if we have no upstreams return
  if (!node.upstream_calls) {
    return { nodes: nodes, links: links };
  }

  // process child nodes
  for (var i = 0; i < node.upstream_calls.length; i++) {
    var nodeX = xStart + (incrX * i);
    var nodeY = yStart + incrY;

    // process the child nodes
    var n = processNode(node.upstream_calls[i], node, nodeX, nodeY);
    nodes = nodes.concat(n.nodes);
    links = links.concat(n.links);

    // add a port for each node
    data.ports["output" + i] = { id: "output" + i, type: "output" };

    // add the link
    links.push({
      id: node.name + i,
      from: {
        nodeId: node.name,
        portId: "output" + i
      },
      to: {
        nodeId: node.upstream_calls[i].name,
        portId: "input0"
      }
    });
  }

  return { nodes: nodes, links: links };
}

export const processData = (APIData) => {
  var data = {}
  data.nodes = {};
  data.links = {};
  data.selected = {};
  data.hovered = {};
  data.offset = {
    x: 0,
    y: 0
  };


  // create create the nodes
  var nd = processNode(APIData, null, startX, startY);

  // create the correct node structure from the flat array
  for (var n = 0; n < nd.nodes.length; n++) {
    data.nodes[nd.nodes[n].id] = nd.nodes[n];
  }

  for (var l = 0; l < nd.links.length; l++) {
    data.links[nd.links[l].id] = nd.links[l];
  }

  return data;
}