require "spec_helper"

describe StressTesters do
  let(:pid) { 23 }
  let(:wait_thread) { double(:fake_wait_thread, value: 0) }
  let(:instance_id) do
    get "/id"
    last_response.body
  end

  before do
    @spawn_commands = []
    @open3_commands = []
    allow(Process).to receive(:spawn) do |command, options|
      @spawn_commands << {command: command, options: options}
      pid
    end
    allow(Process).to receive(:detach)

    allow(Open3).to receive(:popen2e) do |command, &block|
      @open3_commands << command
      output = @spawn_commands.map { |command| command[:command] }.join("\n")
      block.call nil, StringIO.new(output), wait_thread
    end
  end

  describe "GET/POST /instances/:instance_id/stress_testers" do
    it "should detach the process" do
      expect(Process).to receive(:detach).with(pid)
      post "/instances/#{instance_id}/stress_testers"
    end

    it "creates a new stress process with default params" do
      get "/instances/#{instance_id}/stress_testers"
      expect(last_response.status).to eq 200
      expect(last_response.body).to eq ""
      expect(@open3_commands).to include("pgrep stress | xargs -r ps -H")

      post "/instances/#{instance_id}/stress_testers"
      expect(last_response.status).to eq 201
      expect(@spawn_commands).to include(command: "./stress", options: {in: "/dev/null", out: "/dev/null", err: "/dev/null"})

      get "/instances/#{instance_id}/stress_testers"
      expect(last_response.status).to eq 200
      expect(last_response.body).to include "stress"
    end

    context "when calling this not on ourselves" do
      let(:instance_id) { "other_instance" }

      it "should not start a new stress instance" do
        post "/instances/#{instance_id}/stress_testers"
        expect(last_response.status).to eq 409
      end

      it "should not return anything" do
        get "/instances/#{instance_id}/stress_testers"
        expect(last_response.status).to eq 409
      end
    end

    context "when trying to customize the load" do
      %w[timeout cpu io vm vm-bytes vm-stride vm-hang vm-keep hdd hdd-bytes].each do |option|
        it "invokes stress with specified #{option} load" do
          post "/instances/#{instance_id}/stress_testers?#{option}=23"
          expect(last_response.status).to eq 201

          get "/instances/#{instance_id}/stress_testers"
          expect(last_response.status).to eq 200
          expect(last_response.body).to include "stress --#{option} 23"
        end
      end

      it "when invoking with more than one option" do
        post "/instances/#{instance_id}/stress_testers?vm=33&cpu=22"
        expect(last_response.status).to eq 201

        get "/instances/#{instance_id}/stress_testers"
        expect(last_response.status).to eq 200
        expect(last_response.body).to include "stress"
        expect(last_response.body).to include "--vm 33"
        expect(last_response.body).to include "--cpu 22"
      end

      it "copes when an invalid option is requested" do
        post "/instances/#{instance_id}/stress_testers?bad_option=22"
        expect(last_response.status).to eq 412
      end
    end

    context "when the pgrep command fails" do
      let(:wait_thread) { double(:fake_wait_thread, value: 1) }

      it "copes with invalid commands" do
        get "/instances/#{instance_id}/stress_testers"
        expect(last_response.status).to eq 500
      end
    end
  end

  describe "DELETE /instances/:instance_id/stress_testers" do
    it "stops a stress process on the instance" do
      delete "/instances/#{instance_id}/stress_testers"
      expect(last_response.status).to eq 200
      expect(@open3_commands).to include("pkill stress")
    end

    context "when the pkill command fails" do
      let(:wait_thread) { double(:fake_wait_thread, value: 1) }

      it "copes with invalid commands" do
        delete "/instances/#{instance_id}/stress_testers"
        expect(last_response.status).to eq 500
      end
    end
  end
end