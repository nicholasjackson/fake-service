const startX = 50;
const startY = 230;
const incrX = 400;
const incrY = 400;

function processNode(node, name, parent, level, index, xStart, yStart) {
  var nodes = [];
  var links = [];

  var data = {
    id: node.name + "_" + level.toString() + "_" + index,
    properties: {
      name: node.name,
      body: node.body,
      upstream_address: name,
      ip_addresses: node.ip_addresses,
      duration: node.duration,
      type: node.type,
      response: node.code,
      uri: node.uri,
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

  // if we have no Upstreams return
  if (!node.upstream_calls) {
    return { nodes: nodes, links: links };
  }

  // process child nodes
  var i = 0;
  for (const [key, value] of Object.entries(node.upstream_calls)) {
    var nodeX = xStart + (incrX * i);
    var nodeY = yStart + incrY;
    var nextLevel = parseInt(level) + 1;

    // process the child nodes
    var n = processNode(value, key, node, nextLevel, i, nodeX, nodeY);
    nodes = nodes.concat(n.nodes);
    links = links.concat(n.links);

    // add a port for each node
    data.ports["output" + i] = { id: "output" + i, type: "output" };

    // add the link
    links.push({
      id: node.name + "_" + level + "_" + index + "_" + i,
      from: {
        nodeId: node.name + "_" + level + "_" + index,
        portId: "output" + i
      },
      to: {
        nodeId: value.name + "_" + nextLevel + "_" + i,
        portId: "input0"
      }
    });

    i++;
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
  var nd = processNode(APIData, "", null, 0, 0, startX, startY);

  // create the correct node structure from the flat array
  for (var n = 0; n < nd.nodes.length; n++) {
    data.nodes[nd.nodes[n].id] = nd.nodes[n];
  }

  for (var l = 0; l < nd.links.length; l++) {
    data.links[nd.links[l].id] = nd.links[l];
  }

  return data;
}