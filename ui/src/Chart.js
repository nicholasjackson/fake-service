import { Chart } from "react-google-charts";
import React from 'react'

class Timeline extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      url: props.url,
    };
  }

  componentDidMount() {
    this.fetchData();
  }

  fetchData() {
    fetch(this.state.url)
      .then(res => res.json())
      .then(
        (result) => {
          console.log("response from API:", result);

          // build the data
          var data = [
            [
              { type: 'string', id: 'ID' },
              { type: 'string', id: 'Service' },
              { type: 'date', id: 'Start' },
              { type: 'date', id: 'End' },
            ]
          ];

          // add the data from the nodes
          data.push(this.parseElements(result, data));

          this.setState({
            ...this.state,
            data: data,
          });
        },
        (error) => {
          console.error("error processing API", error);
        }
      );
  }

  parseElements(result, data) {
    // add the root node
    data.push([result.name, Date.parse(result.start_time), Date.parse(result.end_time)]);

    if (!result.upstream_calls) {
      console.log("No upstreams");
      return data;
    }

    // add sub nodes
    for (var i = 0; i < result.upstream_calls.length; i++) {
      data.push(this.parseElements(result.upstream_calls[i], data));
    }

    return data;
  }

  render() {
    return <Chart
      width={'500px'}
      height={'300px'}
      chartType="Timeline"
      loader={<div>Loading Chart</div>}
      data={this.state.data}
    />
  }
}

export default Timeline