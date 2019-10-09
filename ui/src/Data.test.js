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
  upstream_calls: [
    {
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
    {
      name: "Upstream3",
      type: "gRPC",
      start_time: "2019-10-08T17:44:04.558117",
      end_time: "2019-10-08T17:44:04.558187",
      duration: "100ms"
    }
  ]
}


it('adds the correct offset', () => {
  var data = processData(singleNode);

  expect(data.offset).toStrictEqual({ x: 0, y: 0 });
});

it('adds the correct position for parent node', () => {
  var data = processData(multipleNode);

  expect(data.nodes.Service.position.x).toEqual(0);
  expect(data.nodes.Service.position.y).toEqual(0);
});

it('adds the correct position for child nodes', () => {
  var data = processData(multipleNode);

  expect(data.nodes.Service.position.x).toEqual(0);
  expect(data.nodes.Service.position.y).toEqual(0);

  expect(data.nodes.Upstream.position.x).toEqual(0);
  expect(data.nodes.Upstream.position.y).toEqual(300);

  expect(data.nodes.Upstream3.position.x).toEqual(300);
  expect(data.nodes.Upstream3.position.y).toEqual(300);

  expect(data.nodes.Upstream2.position.x).toEqual(0);
  expect(data.nodes.Upstream2.position.y).toEqual(600);

});

it('adds a node with the correct details', () => {
  var data = processData(singleNode);

  expect(data.nodes.Service.id).toEqual("Service");

  // properties
  expect(data.nodes.Service.properties.name).toEqual("Service");
  expect(data.nodes.Service.properties.duration).toEqual("70.219µs");
  expect(data.nodes.Service.properties.type).toEqual("HTTP");
});

it('returns multiple nodes', () => {
  var data = processData(multipleNode);

  expect(data.nodes.Service).toBeDefined();
  expect(data.nodes.Upstream).toBeDefined();
});

it('defines output ports when there are upstreams', () => {
  var data = processData(multipleNode);

  expect(data.nodes.Service.ports.output0).toBeDefined();
  expect(data.nodes.Service.ports.output0.id).toEqual("output0");
  expect(data.nodes.Service.ports.output0.type).toEqual("output");
});

it('defines input ports when there is a parent node', () => {
  var data = processData(multipleNode);

  expect(data.nodes.Upstream.ports.input0).toBeDefined();
  expect(data.nodes.Upstream.ports.input0.id).toEqual("input0");
  expect(data.nodes.Upstream.ports.input0.type).toEqual("input");
});

it('defines links correctly', () => {
  var data = processData(multipleNode);

  expect(data.links.Service0).toBeDefined();
  expect(data.links.Service0.id).toEqual('Service0');
  expect(data.links.Service0.from.nodeId).toEqual('Service');
  expect(data.links.Service0.from.portId).toEqual('output0');
  expect(data.links.Service0.to.nodeId).toEqual('Upstream');
  expect(data.links.Service0.to.portId).toEqual('input0');
});