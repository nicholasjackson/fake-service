import React from 'react';
import ReactDOM from 'react-dom';
import { processData } from './Data'

const singleNode = {
  name: "Service", type: "HTTP", start_time: "2019-10-08T17:44:04.558117", end_time: "2019-10-08T17:44:04.558187", duration: "70.219µs"
}

const multipleNode = {
  name: "Service",
  type: "HTTP",
  start_time: "2019-10-08T17:44:04.558117",
  end_time: "2019-10-08T17:44:04.558187",
  duration: "70.219µs",
  upstream_calls: {
    "Upstream": {
      name: "Upstream",
      type: "gRPC",
      start_time: "2019-10-08T17:44:04.558117",
      end_time: "2019-10-08T17:44:04.558187",
      duration: "100ms",
      upstream_calls: [
        {
          name: "Upstream2",
          type: "HTTP",
          start_time: "2019-10-08T17:44:04.558117",
          end_time: "2019-10-08T17:44:04.558187",
          duration: "100ms",
        }
      ]
    },
    "Upstream3": {
      name: "Upstream3",
      type: "gRPC",
      start_time: "2019-10-08T17:44:04.558117",
      end_time: "2019-10-08T17:44:04.558187",
      duration: "100ms"
    }
  }
}

it('adds the correct offset', () => {
  var data = processData(singleNode);

  expect(data.offset).toStrictEqual({ x: 0, y: 0 });
});

it('returns multiple nodes', () => {
  var data = processData(multipleNode);
  
  expect(data.nodes.Service_0_0).toBeDefined();
  expect(data.nodes.Upstream_1_0).toBeDefined();
});

it('adds the correct position for parent node', () => {
  var data = processData(multipleNode);

  expect(data.nodes.Service_0_0.position.x).toEqual(100);
  expect(data.nodes.Service_0_0.position.y).toEqual(100);
});

it('adds the correct position for child nodes', () => {
  var data = processData(multipleNode);

  expect(data.nodes.Upstream_1_0.position.x).toEqual(100);
  expect(data.nodes.Upstream_1_0.position.y).toEqual(400);

  expect(data.nodes.Upstream3_1_1.position.x).toEqual(500);
  expect(data.nodes.Upstream3_1_1.position.y).toEqual(400);

  expect(data.nodes.Upstream2_2_0.position.x).toEqual(100);
  expect(data.nodes.Upstream2_2_0.position.y).toEqual(700);

});

it('adds a node with the correct details', () => {
  var data = processData(singleNode);

  expect(data.nodes.Service_0_0.id).toEqual("Service_0_0");

  // properties
  expect(data.nodes.Service_0_0.properties.name).toEqual("Service");
  expect(data.nodes.Service_0_0.properties.duration).toEqual("70.219µs");
  expect(data.nodes.Service_0_0.properties.type).toEqual("HTTP");
});


it('defines output ports when there are upstreams', () => {
  var data = processData(multipleNode);

  expect(data.nodes.Service_0_0.ports.output0).toBeDefined();
  expect(data.nodes.Service_0_0.ports.output0.id).toEqual("output0");
  expect(data.nodes.Service_0_0.ports.output0.type).toEqual("output");
});

it('defines input ports when there is a parent node', () => {
  var data = processData(multipleNode);


  expect(data.nodes.Upstream_1_0.ports.input0).toBeDefined();
  expect(data.nodes.Upstream_1_0.ports.input0.id).toEqual("input0");
  expect(data.nodes.Upstream_1_0.ports.input0.type).toEqual("input");
});

it('defines links correctly', () => {
  var data = processData(multipleNode);

  expect(data.links.Service_0_0_0).toBeDefined();
  expect(data.links.Service_0_0_0.id).toEqual('Service_0_0_0');
  expect(data.links.Service_0_0_0.from.nodeId).toEqual('Service_0_0');
  expect(data.links.Service_0_0_0.from.portId).toEqual('output0');
  expect(data.links.Service_0_0_0.to.nodeId).toEqual('Upstream_1_0');
  expect(data.links.Service_0_0_0.to.portId).toEqual('input0');
});
